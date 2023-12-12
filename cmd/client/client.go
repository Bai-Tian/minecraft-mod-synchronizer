package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/Bai-Tian/minecraft-mod-synchronizer/cmd"
)

const (
	ServerAddress    = "http://localhost:25555"
	ListEndpoint     = "/list"
	DownloadEndpoint = "/download"
	DownloadPrompt   = "确认操作？（y/n）："
	ConfigFile       = "客户端mod同步器.json"
)

type Config struct {
	ModsFolder string `json:"mods_folder"`
}

var modList ModList

type Mod struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
}

type ModList struct {
	Mods   []Mod             `json:"mods"`
	Delete map[string]string `json:"delete"`
}

func Exit() {
	fmt.Println("\n按 Enter 键退出...")
	fmt.Scanln()
	os.Exit(1)
}

func findModsFolders(path string) ([]string, error) {
	var modsFolders []string
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == "mods" {
			modsFolders = append(modsFolders, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return modsFolders, nil
}

func main() {
	var config Config

	settings, err := cmd.ReadClientSettings()
	if err != nil {
		fmt.Printf("错误：无法读取设置文件 - %v", err)
		Exit()
	}

	// 检查是否存在 客户端mod同步器.json 文件
	if _, err := os.Stat(settings.ConfigFile); os.IsNotExist(err) {
		// 如果不存在，找出所有名为 "mods" 的文件夹
		modsFolders, err := findModsFolders(".")
		if err != nil {
			fmt.Printf("错误：无法找到 mods 文件夹 - %v", err)
			Exit()
		}

		// 询问用户哪个文件夹是他们需要的
		fmt.Println("找到以下 mods 文件夹：")
		for i, folder := range modsFolders {
			fmt.Printf("  %d. %s\n", i+1, folder)
		}
		fmt.Print("请输入你需要的 mods 文件夹的编号：")
		var index int
		fmt.Scanln(&index)

		// 保存用户的选择
		config.ModsFolder = modsFolders[index-1]
		configBytes, err := json.Marshal(config)
		if err != nil {
			fmt.Printf("错误：无法保存配置 - %v", err)
			Exit()
		}
		err = ioutil.WriteFile(settings.ConfigFile, configBytes, 0644)
		if err != nil {
			fmt.Printf("错误：无法写入配置文件 - %v", err)
			Exit()
		}
	} else {
		// 如果存在，读取配置文件
		configBytes, err := ioutil.ReadFile(settings.ConfigFile)
		if err != nil {
			fmt.Printf("错误：无法读取配置文件 - %v", err)
			Exit()
		}
		err = json.Unmarshal(configBytes, &config)
		if err != nil {
			fmt.Printf("错误：无法解析配置文件 - %v", err)
			Exit()
		}
	}

	// 从服务器获取 mod 列表
	resp, err := http.Get(settings.ServerAddress + settings.ListEndpoint)
	if err != nil {
		fmt.Printf("错误：无法从服务器获取 mod 列表 - %v", err)
		Exit()
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("错误：无法读取服务器响应 - %v", err)
		Exit()
	}

	err = json.Unmarshal(body, &modList)
	if err != nil {
		fmt.Printf("错误：无法解析服务器响应 - %v", err)
		Exit()
	}

	// 计算 .minecraft/mods 文件夹下的所有文件的 SHA256 哈希值和文件名
	localMods := make(map[string]string)
	err = filepath.Walk(config.ModsFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			hash := sha256.Sum256(data)
			hashStr := hex.EncodeToString(hash[:])
			localMods[info.Name()] = hashStr
		}

		return nil
	})

	if err != nil {
		fmt.Printf("错误：无法计算文件的哈希值 - %v", err)
		Exit()
	}

	// 检查是否有需要删除的 mod
	var modsToDelete []string
	for modName, modHash := range modList.Delete {
		if localHash, ok := localMods[modName]; ok && localHash == modHash {
			modsToDelete = append(modsToDelete, modName)
		}
	}

	if len(modsToDelete) > 0 {
		fmt.Printf("以下 mod 需要删除：%v\n", modsToDelete)
		fmt.Printf("%s\n", settings.DownloadPrompt)
		var input string
		fmt.Scanln(&input)
		if input == "y" {
			for _, modName := range modsToDelete {
				err := os.Remove(filepath.Join(config.ModsFolder, modName))
				if err != nil {
					log.Printf("错误：无法删除 mod %s - %v", modName, err)
				} else {
					log.Printf("已删除 mod：%s", modName)
				}
			}
		}
	}

	// 检查是否有需要下载的 mod
	var modsToDownload []Mod
	for _, mod := range modList.Mods {
		if localHash, ok := localMods[mod.Name]; !ok || localHash != mod.Hash {
			modsToDownload = append(modsToDownload, mod)
		}
	}

	if len(modsToDownload) > 0 {
		fmt.Println("以下 mod 需要下载：")
		for _, mod := range modsToDownload {
			fmt.Printf("  - %s\n", mod.Name)
		}
		fmt.Printf("%s\n", settings.DownloadPrompt)
		var input string
		fmt.Scanln(&input)
		if input == "y" {
			var wg sync.WaitGroup
			downloadCh := make(chan Mod, 10) // 同时下载 10 个 mod 文件

			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for mod := range downloadCh {
						escapedModName := url.QueryEscape(mod.Name)
						resp, err := http.Get(settings.ServerAddress + settings.DownloadEndpoint + "?file=" + escapedModName)
						if err != nil {
							log.Printf("下载队列 %d：错误：无法下载 mod %s - %v", id, mod.Name, err)
							continue
						}
						defer resp.Body.Close()

						out, err := os.Create(filepath.Join(config.ModsFolder, mod.Name))
						if err != nil {
							log.Printf("下载队列 %d：错误：无法创建 mod 文件 %s - %v", id, mod.Name, err)
							continue
						}
						defer out.Close()

						_, err = io.Copy(out, resp.Body)
						if err != nil {
							log.Printf("下载队列 %d：错误：无法保存 mod 文件 %s - %v", id, mod.Name, err)
							continue
						}

						log.Printf("下载完成：%s", mod.Name)
					}
				}(i)
			}

			for i, mod := range modsToDownload {
				downloadCh <- mod
				log.Printf("(%d/%d) 已加入下载队列：%s", i+1, len(modsToDownload), mod.Name)
			}

			close(downloadCh)
			wg.Wait()
			fmt.Println("所有 mod 已更新完毕。")
		}
	} else if len(modsToDelete) == 0 {
		fmt.Println("所有 mod 都是最新的，无需更新。")
	}

	Exit()
}
