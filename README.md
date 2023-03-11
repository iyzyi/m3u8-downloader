# M3U8下载器

学了3天golang，写个小工具稍微练习一下熟练度。

这个程序基本上包含了绝大多数golang中常用的语法，有些地方其实完全可以简洁一点，但我为了尽可能用到更多的知识点，刻意写得臃肿一些:-)

## 参数

| 参数      | 含义                   | 备注                  |
| --------- | ---------------------- | --------------------- |
| URL       | m3u8视频切片列表的网址 | 必选                  |
| RootPath  | 保存的根目录           | 必选                  |
| ThreadNum | 并发下载的协程数量     | 可选，默认8           |
| FileName  | 保存的视频文件名称     | 可选，默认从URL中提取 |
| ProxyURL  | http代理               | 可选，默认不开启代理  |

