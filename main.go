package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var Config map[string]interface{}

func main() {
	for {

		if _, err := readConfig(); err != nil {
			if _, err := writeLog("Config： " + err.Error()); err != nil {
				log.Fatal(err)
			}
			return
		}

		cycle := Config["cycle"]
		if cycle == nil {
			if _, err := writeLog("Config： the param cycle is nil"); err != nil {
				log.Fatal(err)
			}
			return
		}
		if _, err := spider(); err != nil {
			if _, err := writeLog("Spider： " + err.Error()); err != nil {
				log.Fatal(err)
			}
			return
		}
		//循环执行周期
		fmt.Println("Wait......")
		if _, err := writeLog("Wait： execute again in " + strconv.FormatFloat(cycle.(float64), 'f', -1, 64) + " seconds"); err != nil {
			log.Fatal(err)
		}

		time.Sleep(time.Duration(cycle.(float64)) * time.Second)
	}
}

/*读取配置文件*/
func readConfig() (int, error) {
	file, err := os.Open("config.ini") // For read access.
	if err != nil {
		return 1, err
	}

	var jsonStr string
	buf := bufio.NewReader(file)
	for {
		line, err := buf.ReadString('\n')
		line = strings.TrimSpace(line)
		//移除UTF-8 BOM头
		if len(line) > 2 {
			if line[0] == 239 && line[1] == 187 && line[2] == 191 {
				line = line[3:]
			}
		}
		//自定义单行注释规则,(以#号开头）
		if len(line) > 0 {
			if line[0] == 35 {
				line = ""
			}
		}
		jsonStr = jsonStr + line
		if err == io.EOF {
			break
		}
	}

	//json str 转map
	var dat map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &dat); err != nil {
		return 2, err
	}

	//fmt.Println(dat)
	Config = make(map[string]interface{})
	for k, v := range dat {
		Config[k] = v
	}
	return 0, nil

}

/*写入日志*/
func writeLog(msg string) (int, error) {
	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile("runtime.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 1, err
	}
	if _, err := f.Write([]byte(time.Now().Format("2006/01/02 15:04:05") + "  " + msg + "\r\n"));
		err != nil {
		return 2, err
	}
	if err := f.Close(); err != nil {
		return 3, err
	}
	return 0, nil
}

/*爬虫*/
func spider() (int, error) {
	url := Config["url"].([]interface{})
	if url == nil {
		return 1, errors.New("the param url is nil")
	}

	for k := range url {
		fmt.Println("Get the host " + url[k].(string))
		if _, err := writeLog("Spider： Get the host " + url[k].(string) + ""); err != nil {
			log.Fatal(err)
		}
		tr := &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: true,
		}
		client := &http.Client{Transport: tr}
		resp, err := client.Get(url[k].(string))
		if err != nil {
			if _, err := writeLog("Exception： " + err.Error()); err != nil {
				log.Fatal(err)
			}
			//请求异常,发送钉钉消息
			if _, err := dingtalk(err.Error(), url[k].(string)); err != nil {
				if _, err := writeLog("Dingtalk： " + err.Error()); err != nil {
					log.Fatal(err)
				}
			}
			continue
		}
		if resp.StatusCode != 200 {
			if _, err := writeLog("Exception: Get " + url[k].(string) + " response Status is" + resp.Status); err != nil {
				log.Fatal(err)
			}
			//站点异常,发送钉钉消息
			if _, err := dingtalk("Exception: Get "+url[k].(string)+" response Status is "+resp.Status, url[k].(string)); err != nil {
				if _, err := writeLog("Dingtalk: " + err.Error()); err != nil {
					log.Fatal(err)
				}
			}
			continue
		}
	}
	return 0, nil
}

/*钉钉通知*/
func dingtalk(msg string, url string) (int, error) {
	dingtalk := Config["dingtalk"].(map[string]interface{})
	if dingtalk["token"] == nil {
		return 1, errors.New("the param token is nil")
	}
	if dingtalk["at"] == nil {
		return 1, errors.New("the param token is nil")
	}

	token := dingtalk["token"].(string)
	at := dingtalk["at"].(map[string]interface{})

	var atMobiles []interface{}
	var isAtAll interface{}
	if dingtalk["atMobiles"] == nil {
		atMobiles = make([]interface{}, 0)
	} else {
		atMobiles = at["atMobiles"].([]interface{})

	}
	if dingtalk["isAtAll"] == nil {
		isAtAll = false
	} else {
		isAtAll = at["isAtAll"].(interface{})

	}

	atMobileArray := make([]string, len(atMobiles))
	atMobileStr := ""
	for k := range atMobiles {
		atMobileArray[k] = atMobiles[k].(string)
		atMobileStr = "@" + atMobiles[k].(string)
	}
	atMobilejson, _ := json.Marshal(atMobileArray)
	param := `{"msgtype":"markdown","markdown": {"title": "网站监控通知\n\r ","text": "网站监控通知\n\r错误类型：` + msg + `\n\r错误地址：` + url + `\n\r错误时间：` + time.Now().Format("2006/01/02 15:04:05") + `\n\r` + atMobileStr + `"},"at": {"atMobiles": ` + string(atMobilejson) + `,"isAtAll": "` + strconv.FormatBool(isAtAll.(bool)) + `"}}`

	webhook := "https://oapi.dingtalk.com/robot/send?access_token=" + token
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Post(webhook, "application/json",
		strings.NewReader(param))
	if err != nil {
		return 2, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 3, err
	}
	if _, err := writeLog("Dingtalk: " + param + " " + string(body)); err != nil {
		log.Fatal(err)
	}
	return 0, nil
}

/*获取变量类型*/
func typeof(v interface{}) string {
	return reflect.TypeOf(v).String()
}
