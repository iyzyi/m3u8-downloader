package M3U8

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/schollz/progressbar/v3"
)

// go env -w GO111MODULE=off

type M3U8Downloader struct {
	URL       string
	RootPath  string
	FileName  string
	ThreadNum int
	ProxyURL  string

	tempPath       string
	filePath       string
	tempMergeNames []string
	sliceNames     []string
	sliceParams    []string
	sliceLinks     []string
	sliceNum       int
	bar            *progressbar.ProgressBar
	client         *http.Client
}

var waitGroup = new(sync.WaitGroup)

func (m3u8 *M3U8Downloader) init() bool {
	fileName, err := getFileNameFromURL(m3u8.URL)
	if err != nil {
		fmt.Printf("错误：%v\n", err)
		return false
	}

	if m3u8.FileName == "" {
		if fileName[len(fileName)-5:] == ".m3u8" {
			m3u8.FileName = fileName[:len(fileName)-5] + ".mp4"
		} else {
			m3u8.FileName = fileName + ".mp4"
		}
	}

	if m3u8.ThreadNum == 0 {
		m3u8.ThreadNum = 8
	}

	if m3u8.ProxyURL != "" {
		uri, _ := url.Parse(m3u8.ProxyURL)
		m3u8.client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(uri),
			},
		}
	} else {
		m3u8.client = nil
	}

	fileName = strings.Replace(fileName, ".", "_", -1)
	fileName = strings.Replace(fileName, "-", "_", -1)
	m3u8.tempPath = path.Join(m3u8.RootPath, "__temp_"+fileName)

	if !pathExits(m3u8.tempPath) {
		os.MkdirAll(m3u8.tempPath, os.ModePerm)
	}

	m3u8.filePath = path.Join(m3u8.tempPath, m3u8.FileName)

	return true
}

func (m3u8 *M3U8Downloader) getWebData(url string) *[]uint8 {
	var res *http.Response
	var err error
	if m3u8.client == nil {
		res, err = http.Get(url)
	} else {
		res, err = m3u8.client.Get(url)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v", err)
		return nil
	}

	body, err := io.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v", err)
		return nil
	}

	return &body
}

func (m3u8 *M3U8Downloader) joinVedioSliceLink(param string) string {
	reg := regexp.MustCompile(`(https?|ftp|file)://[-A-Za-z0-9+&@#/%?=~_|!:,.;]+[-A-Za-z0-9+&@#/%=~_|]`)
	if reg.MatchString(param) {
		return param
	}

	basePath, _ := filepath.Split(m3u8.URL)
	if basePath[len(basePath)-1] != '/' && basePath[len(basePath)-1] != '\\' {
		basePath += "/"
	}
	link := basePath + param
	return link
}

func (m3u8 *M3U8Downloader) getVedioSliceLink() bool {
	data := m3u8.getWebData(m3u8.URL)
	if data == nil {
		return false
	}

	text := string(*data)
	if !checkM3U8Data(text) {
		return false
	}

	reg := regexp.MustCompile(`(^|\n)(?P<name>[^#].+?)(\n|$)`)
	if reg == nil {
		return false
	}

	res := reg.FindAllStringSubmatch(text, -1)
	for _, item := range res {
		param := item[2]
		name, _ := getFileNameFromURL(param)
		link := m3u8.joinVedioSliceLink(param)
		m3u8.sliceNames = append(m3u8.sliceNames, name)
		m3u8.sliceParams = append(m3u8.sliceParams, param)
		m3u8.sliceLinks = append(m3u8.sliceLinks, link)
	}

	m3u8.sliceNum = len(m3u8.sliceLinks)
	return m3u8.sliceNum != 0
}

func (m3u8 *M3U8Downloader) joinVedioSlicePath(url string) string {
	fileName, _ := getFileNameFromURL(url)
	slicePath := filepath.Join(m3u8.tempPath, fileName)
	//fmt.Println(slicePath)
	return slicePath
}

func (m3u8 *M3U8Downloader) saveVedioSlice(threadID int, jobs chan string) {
	//fmt.Printf("Thread %d is beginning.\n", threadID)

	// for {
	// 	link, ok := <-jobs
	// 	if !ok {
	// 		break
	// 	}
	// 	fmt.Println(threadID, link)
	// }

	for link := range jobs {
		data := m3u8.getWebData(link)
		if data == nil {
			fmt.Printf("下载失败：%s\n", link)
		} else {
			slicePath := m3u8.joinVedioSlicePath(link)
			err := ioutil.WriteFile(slicePath, *data, 0777)
			if err != nil {
				fmt.Printf("保存失败：%v\n", err)
			} else {
				m3u8.bar.Add(1)
			}
		}
	}

	//fmt.Printf("Thread %d is exiting.\n", threadID)
	waitGroup.Done()
}

func (m3u8 *M3U8Downloader) checkAllSliceExist() bool {
	result := true
	for _, sliceName := range m3u8.sliceParams {
		slicePath := filepath.Join(m3u8.tempPath, sliceName)
		if !pathExits(slicePath) {
			result = false
			break
		}
	}
	return result
}

func (m3u8 *M3U8Downloader) mergeVedioSlice() {
	sliceNumPerGroup := 100

	for begin := 0; begin < m3u8.sliceNum; begin += sliceNumPerGroup {
		var end int
		if begin+sliceNumPerGroup > m3u8.sliceNum {
			end = m3u8.sliceNum
		} else {
			end = begin + sliceNumPerGroup
		}

		tempMergeName := fmt.Sprintf("__%08d.ts", begin/sliceNumPerGroup+1)
		m3u8.tempMergeNames = append(m3u8.tempMergeNames, tempMergeName)

		mergeVideoSlice(m3u8.sliceNames[begin:end], tempMergeName, m3u8.tempPath)
	}

	mergeVideoSlice(m3u8.tempMergeNames, m3u8.FileName, m3u8.tempPath)
}

func (m3u8 *M3U8Downloader) deleteTempVideoSlice() {
	cmd := "del *.ts"
	execCMD(cmd, m3u8.tempPath)
}

func (m3u8 *M3U8Downloader) GetOutputFilePath() string {
	return path.Join(m3u8.tempPath, m3u8.FileName)
}

func (m3u8 *M3U8Downloader) SaveM3U8() bool {
	if !m3u8.init() {
		fmt.Println("初始化失败")
		return false
	}

	if !m3u8.getVedioSliceLink() {
		fmt.Println("获取视频切片链接失败")
		return false
	}

	m3u8.bar = progressbar.Default(int64(m3u8.sliceNum))

	jobs := make(chan string, m3u8.sliceNum)
	for _, link := range m3u8.sliceLinks {
		jobs <- link
	}
	// 如果 chan 关闭前，buffer 内有元素还未读 , 会正确读到 chan 内的值，且返回的第二个 bool 值（是否读成功）为 true。
	// 如果 chan 关闭前，buffer 内有元素已经被读完，chan 内无值，接下来所有接收的值都会非阻塞直接成功，返回 channel 元素的零值，但是第二个 bool 值一直为 false。
	close(jobs)

	var min int
	if m3u8.ThreadNum < m3u8.sliceNum {
		min = m3u8.ThreadNum
	} else {
		min = m3u8.sliceNum
	}

	for i := 0; i < min; i++ {
		waitGroup.Add(1)
		go m3u8.saveVedioSlice(i, jobs)
	}
	waitGroup.Wait()

	if !m3u8.checkAllSliceExist() {
		fmt.Println("M3U8视频切片不全，跳过合并环节")
		return false
	} else {
		m3u8.mergeVedioSlice()
		if !pathExits(m3u8.filePath) {
			return false
		} else {
			m3u8.deleteTempVideoSlice()
			return true
		}
	}
}

func pathExits(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	} else {
		return !os.IsNotExist(err)
	}
}

func getFileNameFromURL(u string) (string, error) {
	res, err := url.Parse(u)
	if err != nil {
		return "", err
	} else {
		_, fileName := path.Split(res.Path)
		return fileName, nil
	}
}

func checkM3U8Data(text string) bool {
	if text[:7] != "#EXTM3U" {
		fmt.Println("提供的URL不属于M3U8文件")
		return false
	}
	if strings.Contains(text, "#EXT-X-STREAM-INF") {
		fmt.Println("提供的URL是M3U8标签文件(提供不同的清晰度、编码参数等的选择)，不是M3U8视频切片列表")
		return false
	}
	return true
}

func execCMD(command string, workPath string) bool {
	// 一个超级大的坑点：
	// https://github.com/golang/go/issues/17149
	// https://blog.csdn.net/a1309525802/article/details/121835317

	// args := strings.Fields("/c " + command)
	// cmd := exec.Command("cmd.exe", args...)
	// if workPath != "" {
	// 	cmd.Dir = workPath
	// }
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	// err := cmd.Run()
	// if err != nil {
	// 	fmt.Printf("cmd.Run() failed with %v\n", err)
	// }

	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: fmt.Sprintf(`/c %s`, command), HideWindow: true}
	//fmt.Println(command)
	cmd.Dir = workPath
	//res, err := cmd.Output()
	_, err := cmd.Output()

	// 执行结果为GBK编码，golang没法直接解析。
	//fmt.Println(res)

	if err != nil {
		fmt.Printf("cmd exec error: %v\n", err)
		return false
	} else {
		return true
	}
}

func hasFfmpegInstalled() bool {
	var command = "ffmpeg -version"
	return execCMD(command, "")
}

// 这里其实没有必要使用接口，但是为了练习语法，就这么搞了:-)
type videoSlices interface {
	merge() bool
}

type copyb struct {
	sliceParams []string
	mergeName   string
	tempPath    string
}

func (this *copyb) merge() bool {
	names := ""
	for i := range this.sliceParams {
		if names == "" {
			names += fmt.Sprintf(`"%s"`, this.sliceParams[i])
		} else {
			names += fmt.Sprintf(`+"%s"`, this.sliceParams[i])
		}
	}

	cmd := fmt.Sprintf(`copy/b %s "%s"`, names, this.mergeName)
	//fmt.Println(cmd)
	execCMD(cmd, this.tempPath)
	return true
}

type ffmpeg struct {
	sliceParams []string
	mergeName   string
	tempPath    string
}

func (this *ffmpeg) merge() bool {
	names := ""
	for i := range this.sliceParams {
		if names == "" {
			names += fmt.Sprintf(`"%s"`, this.sliceParams[i])
		} else {
			names += fmt.Sprintf(`|"%s"`, this.sliceParams[i])
		}
	}

	cmd := fmt.Sprintf(`ffmpeg -i "concat:%s" -y -loglevel quiet -acodec copy -vcodec copy -crf 0 "%s"`, names, this.mergeName)
	//fmt.Println(cmd)
	execCMD(cmd, this.tempPath)
	return true
}

func mergeVideoSlice(sliceParams []string, mergeName string, tempPath string) bool {
	var tool videoSlices
	if hasFfmpegInstalled() {
		tool = &ffmpeg{sliceParams, mergeName, tempPath}
	} else {
		tool = &copyb{sliceParams, mergeName, tempPath}
	}
	return tool.merge()
}
