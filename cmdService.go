package main

import (
	"archive/zip"
	"fmt"
	"github.com/spf13/afero"
	"github.com/tidwall/gjson"
	"gopkg.in/urfave/cli.v1"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//==========================================================================
func main() {
	app := cli.NewApp()
	app.Name = "MADP CLI"
	app.Version = "2.0.0"
	app.Compiled = time.Now()
	app.Authors = []cli.Author{
		cli.Author{
			Name:  "Lu Chengming",
			Email: "luchengming@csii.com.cn",
		},
	}
	app.Copyright = "(c) 2017 Client Service International Inc."
	app.HelpName = "madp-cli"
	app.Usage = "demonstrate available API"
	app.UsageText = "Mobile Application Development Platform - Command-line Interface"
	app.ArgsUsage = "[args and such]"
	app.Commands = []cli.Command{ //命令集
		{
			Name:    "download",
			Aliases: []string{"dl"},
			Usage:   "Launch a service for download file",
			Action:  downloadFile,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name: "find",
				},
				cli.StringFlag{
					Name: "key",
				},
				cli.StringFlag{
					Name: "url",
				},
				cli.StringFlag{
					Name: "uid",
				},
			},
		},
	}
	app.Run(os.Args)
}
func downloadFile(c *cli.Context) error {
	//./main dl -find /Users/liushuai/go/src/fileService/registry.json
	if filePath := c.String("find"); filePath != "" { //读取配置文件的信息
		if registry2s, e := Parse2(filePath); e != nil {
			log.Println(e)
		} else {
			fmt.Printf("%-8s\t%-8s\t%-8s\t%-8s\n", "appid", "versionCode", "versionName", "key")
			fmt.Println("--------------------------------------------------------------------------")
			for _, v := range registry2s {
				fmt.Printf("%-8s\t%-8s\t%-8s\t%-8s\n", v.Appid, v.VersionCode, v.VersionName, v.Key)
			}
		}
	} else {
		//http://10.1.1.167:8081/download?uid=csii&key=12ffdce0e4da40e0ec2f0ba40e3b129e18facfbc0723d8195f35905169aa65ee&filename=registry
		if c.String("key") == "" || c.String("url") == "" || c.String("uid") == "" {
			fmt.Println("Incomplete request path , please check ")
			return nil
		}
		completeUrl := "http://" + c.String("url") + `/download?uid=` + c.String("uid") + `&key=` + c.String("key")
		fmt.Println(`request complete url: `, completeUrl)
		fmt.Println(`downloading...`)
		//下载离线包
		fileName := "download.zip"
		path := GetCurrentDirectory2()
		if err := httpDownlodFile2(fileName, path, completeUrl); err != nil {
			log.Println(err)
			return err
		}
		//解压
		if dirName, err := Unzip2(path+"/"+fileName, path+"/"); err != nil {
			log.Println(err)
			return err
		} else {
			//删除压缩包
			os.RemoveAll(path + "/" + fileName)
			//下载注册表
			fileName = dirName + ".dat"
			completeUrl = completeUrl + "&filename=registry"
			if err := httpDownlodFile2(fileName, path, completeUrl); err != nil {
				log.Println(err)
				return err
			}
		}
		fmt.Println(`download over`)
	}

	return nil
}

//打成执行文件后可获得当前目录-->加入path后，在什么路径下调用，就会拿到什么路径，而不是path路径
func GetCurrentDirectory2() string {
	//返回绝对路径  filepath.Dir(os.Args[0])去除最后一个元素的路径
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return strings.Replace(dir, "\\", "/", -1) //将\替换成/
}

//下载文件
func httpDownlodFile2(fileName, path, url string) (err error) {
	resp, err1 := http.Get(url)
	if err1 != nil {
		err = err1
		return
	}
	defer resp.Body.Close()
	f, err3 := os.Create(path + "/" + fileName)
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

	return
}

//解压zip文件
func Unzip2(zipFile string, destDir string) (string, error) {
	zipReader, err := zip.OpenReader(zipFile)
	if err != nil {
		return "", err
	}
	defer zipReader.Close()

	for _, f := range zipReader.File {
		fpath := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
		} else {
			if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				return "", err
			}

			inFile, err := f.Open()
			if err != nil {
				return "", err
			}
			defer inFile.Close()

			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return "", err
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, inFile)
			if err != nil {
				return "", err
			}
		}
	}
	//获取文件夹名字
	fName := zipReader.File[0].Name
	split := strings.Split(fName, "/")
	i := 0
	for {
		if split[i] == "" {
			i++
		} else {
			fName = split[i]
			break
		}
	}
	return fName, nil
}

type Registry2 struct {
	Appid string `json:"appid"`
	Key   string `json:"key"`
	//Status      string `json:"status"`
	VersionCode string `json:"versionCode"`
	VersionName string `json:"versionName"`
}

func Parse2(path string) ([]Registry2, error) {
	var Registrys []Registry2
	//fileName := "registry.json"
	fs := afero.OsFs{}
	registry, err := afero.ReadFile(fs, path)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	appids := gjson.Get(string(registry), "#.appid")
	keys := gjson.Get(string(registry), "#.key")
	statuss := gjson.Get(string(registry), "#.status")
	versionCodes := gjson.Get(string(registry), "#.versionCode")
	versionNames := gjson.Get(string(registry), "#.versionName")
	for n := range appids.Array() {
		if statuss.Array()[n].String() != "2" {
			continue
		}
		reg := Registry2{}
		reg.Appid = appids.Array()[n].String()
		reg.Key = keys.Array()[n].String()
		//reg.Status = statuss.Array()[n].String()
		reg.VersionCode = versionCodes.Array()[n].String()
		reg.VersionName = versionNames.Array()[n].String()
		Registrys = append(Registrys, reg)
	}
	return Registrys, nil
}

//==========================================================================
