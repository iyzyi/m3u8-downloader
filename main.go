package main

import (
	M3U8 "M3U8/m3u8"
	"fmt"
)

func main() {
	var m3u8 M3U8.M3U8Downloader
	m3u8.URL = "https://xushanxiang.com/demo/ffmpeg/hls265/output.m3u8"
	m3u8.RootPath = `D:\桌面\m3u8下载测试`
	//m3u8.ThreadNum = 32
	//m3u8.FileName = "default.mp4"
	//m3u8.ProxyURL = "http://127.0.0.1:8889"

	success := m3u8.SaveM3U8()
	if success {
		fmt.Printf("下载成功，PATH: %s\n", m3u8.GetOutputFilePath())
	} else {
		fmt.Printf("下载失败，URL: %s\n", m3u8.URL)
	}
}
