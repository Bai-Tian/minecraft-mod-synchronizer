package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Bai-Tian/minecraft-mod-synchronizer/cmd"
	"github.com/spf13/cobra"
)

const (
	ModsFolder       = "mods"
	DeleteFile       = "delete.json"
	ServerPort       = 25555
	ListEndpoint     = "/list"
	DownloadEndpoint = "/download"
)

var settings cmd.ServerSettings

var modList ModList

type Mod struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
}

type ModList struct {
	Mods   []Mod             `json:"mods"`
	Delete map[string]string `json:"delete"`
}

func main() {
	rootCmd := &cobra.Command{
		Use: "server",
		Run: main0,
	}

	rootCmd.PersistentFlags().StringVarP(&settings.ModsFolder, "mods-folder", "m", ModsFolder, "ServerSettings::ModsFolder")
	rootCmd.PersistentFlags().StringVarP(&settings.DeleteFile, "delete-file", "d", DeleteFile, "ServerSettings::DeleteFile")
	rootCmd.PersistentFlags().Uint16VarP(&settings.ServerPort, "server-port", "P", ServerPort, "ServerSettings::ServerPort")
	rootCmd.PersistentFlags().StringVar(&settings.ListEndpoint, "list-endpoint", ListEndpoint, "ServerSettings::ListEndpoint")
	rootCmd.PersistentFlags().StringVar(&settings.DownloadEndpoint, "download-endpoint", DownloadEndpoint, "ServerSettings::DownloadEndpoint")

	err := rootCmd.Execute()
	if err != nil {
		log.Fatalf("invalid args: %v", err)
	}
}

func main0(c *cobra.Command, args []string) {
	// 检查是否存在 mods 文件夹
	if _, err := os.Stat(settings.ModsFolder); os.IsNotExist(err) {
		log.Fatalf("错误：找不到 %s 文件夹", settings.ModsFolder)
	}

	// 检查是否存在 delete.json 文件
	if _, err := os.Stat(settings.DeleteFile); os.IsNotExist(err) {
		// 如果不存在，则创建空的 delete.json 文件
		err := ioutil.WriteFile(settings.DeleteFile, []byte("{}"), 0644)
		if err != nil {
			log.Fatalf("错误：无法创建 %s 文件", settings.DeleteFile)
		}
	}

	// 读取 delete.json 文件
	deleteListBytes, err := ioutil.ReadFile(settings.DeleteFile)
	if err != nil {
		log.Fatalf("错误：无法读取 %s 文件", settings.DeleteFile)
	}

	err = json.Unmarshal(deleteListBytes, &modList.Delete)
	if err != nil {
		log.Fatalf("错误：无法解析 delete.json 文件")
	}

	// 计算 mods 文件夹下的所有文件的 SHA256 哈希值和文件名
	err = filepath.Walk(settings.ModsFolder, func(path string, info os.FileInfo, err error) error {
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
			modList.Mods = append(modList.Mods, Mod{Name: info.Name(), Hash: hashStr})
		}

		return nil
	})

	if err != nil {
		log.Fatalf("错误：无法计算文件的哈希值 - %v", err)
	}

	// 设置 HTTP 请求处理函数
	http.HandleFunc(settings.ListEndpoint, listHandler)
	http.HandleFunc(settings.DownloadEndpoint, downloadHandler)

	// 启动 HTTP 服务器
	serverAddress := fmt.Sprintf(":%d", int(settings.ServerPort))
	log.Printf("服务器已启动，监听地址：%s", serverAddress)
	err = http.ListenAndServe(serverAddress, nil)
	if err != nil {
		log.Fatalf("错误：无法启动服务器 - %v", err)
	}
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	modListBytes, err := json.Marshal(modList)
	if err != nil {
		http.Error(w, "错误：无法生成 mod 列表", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(modListBytes)
	log.Println("已返回 mod 列表")
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	modName := r.URL.Query().Get("file")
	if modName == "" {
		http.Error(w, "错误：缺少文件参数", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(settings.ModsFolder, modName)
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "错误：文件未找到", http.StatusNotFound)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		http.Error(w, "错误：无法获取文件信息", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", modName))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	http.ServeContent(w, r, modName, fileInfo.ModTime(), file)
	log.Printf("已下载 mod：%s", modName)
}
