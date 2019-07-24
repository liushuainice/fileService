package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var AppIds string
var VserionCodes string

const (
	uploadPath = "./tmp"
	//ip            = "10.1.1.22"
	//port          = "8081"
	//userId        = "csii"
	//status        = "2" //允许下载的状态
	leng = 30 //轮巡时间间隔/秒
)

type AppStatus struct {
	AppId       string `json:"appid"`
	VersionCode string `json:"versionCode"`
	VersionName string `json:"versionName"`
	Status      string `json:"status"`
}

func main() {
	//0,读取配置文件
	if err := Parse(); err != nil {
		log.Fatal(err)
	} else {
		log.Println(`---> config: `, *Config)
	}
	//1,判断并创建tmp文件夹
	createFile(uploadPath)
	//2,启动轮训服务，查询下载文件到tmp里
	go DownloadFileService()
	//3,暴露下载路由，外部可以下载静态文件
	http.Handle("/downloads/", http.StripPrefix("/downloads/", http.FileServer(http.Dir(uploadPath))))
	http.HandleFunc("/download/central.dat", Downloads2)
	http.HandleFunc("/", MissRoute2)
	log.Println("Server started on " + GetOutboundIP().String() + ":" + Config.SerPort + ", use  /download/{fileName} for downloading")
	log.Fatal(http.ListenAndServe(":"+Config.SerPort, nil))
}

func main1() {
	//0,读取配置文件
	if err := Parse(); err != nil {
		log.Fatal(err)
	} else {
		log.Println(`---> config: `, *Config)
	}
	//1,判断并创建tmp文件夹
	createFile(uploadPath)
	//2,启动轮训服务，查询下载文件到tmp里
	//go DownloadFileService()
	//3,暴露下载路由，外部可以下载静态文件
	router := gin.Default()
	router.StaticFS("/downloads", http.Dir(uploadPath))
	router.GET("/download/central.dat", Downloads)
	router.NoRoute(MissRoute) //其他所有路由都走这个方法
	log.Println("Server started on " + GetOutboundIP().String() + ":" + Config.SerPort + ", use  /download/{fileName} for downloading")
	log.Fatal(router.Run(":" + Config.SerPort))
}
func GetOutboundIP() net.IP { //获取本机外网IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Println(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

//第二次后文件下载路由
func MissRoute(c *gin.Context) {
	url := c.Request.URL
	newUrl := strings.Replace(url.Path, `/download`, uploadPath+"/"+AppIds+"-"+VserionCodes, 1)
	fmt.Println("==> downloads:", newUrl)
	fp, err := os.OpenFile(newUrl, os.O_RDONLY, 0755)
	if err != nil {
		c.Writer.Write([]byte("error"))
		return
	}
	var datas []byte
	for {
		data := make([]byte, 4096)
		n, err := fp.Read(data)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Println("read file err:", err)
			c.Writer.Write([]byte("error"))
			return
		}
		datas = append(datas, data[:n]...)
	}

	c.Writer.Write(datas)
}
func MissRoute2(w http.ResponseWriter, r *http.Request) {
	url := r.URL
	newUrl := strings.Replace(url.Path, `/download`, uploadPath+"/"+AppIds+"-"+VserionCodes, 1)
	fmt.Println("==> downloads:", newUrl)
	fp, err := os.OpenFile(newUrl, os.O_RDONLY, 0755)
	if err != nil {
		w.Write([]byte("error"))
		return
	}
	var datas []byte
	for {
		data := make([]byte, 4096)
		n, err := fp.Read(data)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Println("read file err:", err)
			w.Write([]byte("error"))
			return
		}
		datas = append(datas, data[:n]...)
	}
	w.WriteHeader(200)
	w.Write(datas)
	//w.Write([]byte(url.Path))

}

//第一次文件下载的路由
func Downloads(c *gin.Context) {
	appId := c.Query("appId")
	vserionCode := c.Query("vserionCode")
	AppIds = appId
	VserionCodes = vserionCode
	fmt.Println("--> downloads:", uploadPath+"/"+appId+"-"+vserionCode+"/central.dat")
	fp, err := os.OpenFile(uploadPath+"/"+appId+"-"+vserionCode+"/central.dat", os.O_RDONLY, 0755)
	if err != nil {
		c.Writer.Write([]byte("error"))
		return
	}
	var datas []byte
	for {
		data := make([]byte, 4096)
		n, err := fp.Read(data)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Println("read file err:", err)
			c.Writer.Write([]byte("error"))
			return
		}
		datas = append(datas, data[:n]...)
	}

	c.Writer.Write(datas)
	c.Status(200)
}

func Downloads2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	appId := r.Form.Get("appId")
	vserionCode := r.Form.Get("vserionCode")
	AppIds = appId
	VserionCodes = vserionCode
	fmt.Println("--> downloads:", uploadPath+"/"+appId+"-"+vserionCode+"/central.dat")
	fp, err := os.OpenFile(uploadPath+"/"+appId+"-"+vserionCode+"/central.dat", os.O_RDONLY, 0755)
	if err != nil {
		w.Write([]byte("error"))
		return
	}
	var datas []byte
	for {
		data := make([]byte, 4096)
		n, err := fp.Read(data)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Println("read file err:", err)
			w.Write([]byte("error"))
			return
		}
		datas = append(datas, data[:n]...)
	}
	w.Write(datas)
}

//从后管下载文件到指定的目录下
func DownloadFileService() {
	url := `http://` + Config.Ip + `:` + Config.Port +
		`/publish/_list.json?userId=` + Config.UserId
	for {
		if b, e := httpGetDB(url); e == nil {
			var apps []AppStatus
			if e := json.Unmarshal(b, &apps); e != nil {
				log.Println(`>>>json.Unmarshal err:`, e, " >>bate: ", string(b), " > url :", url)
				continue
			}
			for _, app := range apps {
				if app.Status == Config.Status {
					fileName := app.AppId + "-" + app.VersionCode + ".zip"
					//判断文件是否存在，存在就continue
					if isExist(uploadPath+"/"+fileName) && getFileStat(uploadPath+"/"+fileName) > 10 {
						log.Println(`>>> `, uploadPath+"/"+fileName, "已存在")
						continue
					}
					log.Println(">>>>> start download file:", fileName)
					url2 := `http://` + Config.Ip + `:` + Config.Port +
						`/publish/file.zip?appId=` + app.AppId +
						`&userId=` + Config.UserId + `&versionCode=` + app.VersionCode
					log.Println(">>>>> start download url:", url2)
					httpDownlodFile(fileName, url2) //下载文件到对应的路径下
				}
			}
		} else {
			log.Println("-->", url, "response error :")
			log.Println(`		`, e, "<--")
		}
		time.Sleep(leng * time.Second)
	}
}

//获得文件状态信息
func getFileStat(file string) int64 {
	fileinfo, err := os.Stat(file)
	if err != nil {
		return int64(0) // 如果首次没有创建则直接返回0
	}
	return fileinfo.Size()
}

//获得json信息
func httpGetDB(url string) (result []byte, err error) {
	resp, err1 := http.Get(url)
	if err1 != nil {
		err = err1
		return
	}
	defer resp.Body.Close()
	buf := make([]byte, 4096)
	for {
		n, err2 := resp.Body.Read(buf)
		if n == 0 {
			break
		}
		if err2 != nil && err2 != io.EOF {
			err = err2
			return
		}
		result = append(result, buf[:n]...)
	}

	return
}

//下载文件
func httpDownlodFile(fileName, url string) (err error) {
	resp, err1 := http.Get(url)
	if err1 != nil {
		err = err1
		return
	}
	defer resp.Body.Close()
	f, err3 := os.Create(uploadPath + "/" + fileName)
	//f, err3 := os.OpenFile(uploadPath+"/"+fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0755)
	if err3 != nil {
		err = err3
		return
	}
	buf := make([]byte, 4096)
	for {
		n, err2 := resp.Body.Read(buf)
		if n == 0 {
			break
		}
		if err2 != nil && err2 != io.EOF {
			err = err2
			return
		}
		f.Write(buf[:n])
	}
	//len(fileName)
	if len(fileName) > 4 && fileName[len(fileName)-3:] == "zip" {
		Unzip(uploadPath+"/"+fileName, uploadPath)
	}
	return
}

//调用os.MkdirAll递归创建文件夹
func createFile(filePath string) error {
	if !isExist(filePath) {
		err := os.MkdirAll(filePath, os.ModePerm)
		return err
	}
	return nil
}

// 判断所给路径文件/文件夹是否存在(返回true是不存在)
func isExist(path string) bool {
	_, err := os.Stat(path) //os.Stat获取文件信息
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

//解压zip文件
func Unzip(zipFile string, destDir string) error {
	zipReader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	for _, f := range zipReader.File {
		fpath := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
		} else {
			if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				return err
			}

			inFile, err := f.Open()
			if err != nil {
				return err
			}
			defer inFile.Close()

			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, inFile)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type config struct {
	Ip      string `json:"ip"`
	Port    string `json:"port"`
	SerPort string `json:"ser_port"`
	UserId  string `json:"user_id"`
	Status  string `json:"status"`
	Leng    int    `json:"leng"`
}

// Config 全局配置
var Config *config

// Parse 指定配置文件filename执行解析
func (c *config) parse(filename string) error {
	file, _ := os.Open(filename)
	decoder := json.NewDecoder(file)
	err := decoder.Decode(&c)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}
	return nil
}

// Parse 初始化配置实例
func Parse() error {
	//flag.Parse() //解析命令行参数使用
	filename := "./file.json"
	/*if flag.Arg(0) != "" {
		filename = flag.Arg(0)
	}*/
	log.Printf("parse config: %s", filename)
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Println("parse config error:", err)
	}
	Config = new(config)
	err := Config.parse(filename)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}
