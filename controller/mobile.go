package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	chinaMobilePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)
	nonDigitPattern    = regexp.MustCompile(`\D`)
	mobileSeparators   = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "")
)

type mobileLookupRequest struct {
	Mobile string `json:"mobile"`
}

type mobileLookupResponse struct {
	Ret  string   `json:"ret"`
	Msg  string   `json:"msg"`
	Data []string `json:"data"`
}

func normalizeChinaMobile(raw string) string {
	mobile := mobileSeparators.Replace(strings.TrimSpace(raw))
	if strings.HasPrefix(mobile, "+86") {
		mobile = mobile[3:]
	} else if strings.HasPrefix(mobile, "86") && len(mobile) > 11 {
		mobile = mobile[2:]
	}
	return nonDigitPattern.ReplaceAllString(mobile, "")
}

func mobileDataAt(data []string, index int) string {
	if index >= len(data) {
		return ""
	}
	return data[index]
}

// PostMobileLookup validates a mainland China mobile number and queries ip138.
// The API token is read from IP138_TOKEN and is never exposed to the browser.
func PostMobileLookup(c *gin.Context) {
	var request mobileLookupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请输入手机号码"})
		return
	}
	mobile := normalizeChinaMobile(request.Mobile)
	if !chinaMobilePattern.MatchString(mobile) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请输入正确的中国大陆 11 位手机号"})
		return
	}

	token := strings.TrimSpace(os.Getenv("IP138_TOKEN"))
	if token == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "msg": "归属地查询服务尚未配置 IP138_TOKEN"})
		return
	}
	baseURL := strings.TrimSpace(os.Getenv("IP138_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.ip138.com/mobile/"
	}
	endpoint, err := url.Parse(baseURL)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "https" && endpoint.Scheme != "http") {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "归属地查询地址配置无效"})
		return
	}
	query := endpoint.Query()
	query.Set("mobile", mobile)
	query.Set("datatype", "json")
	query.Set("token", token)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "无法创建归属地查询请求"})
		return
	}
	client := &http.Client{Timeout: 8 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "msg": "归属地查询服务暂时不可用"})
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "msg": "归属地查询服务返回异常"})
		return
	}

	var payload mobileLookupResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "msg": "归属地查询结果格式无效"})
		return
	}
	if payload.Ret != "ok" {
		message := strings.TrimSpace(payload.Msg)
		if message == "" {
			message = "未查询到归属地信息"
		}
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "msg": message})
		return
	}
	if len(payload.Data) < 2 || (mobileDataAt(payload.Data, 0) == "" && mobileDataAt(payload.Data, 1) == "") {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "号码未收录，未查询到归属地信息"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"result": gin.H{
			"mobile":   mobile,
			"province": mobileDataAt(payload.Data, 0),
			"city":     mobileDataAt(payload.Data, 1),
			"carrier":  mobileDataAt(payload.Data, 2),
			"zone":     mobileDataAt(payload.Data, 3),
		},
	})
}
