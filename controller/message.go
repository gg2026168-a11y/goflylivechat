package controller

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"goflylivechat/common"
	"goflylivechat/models"
	"goflylivechat/tools"
	"goflylivechat/ws"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

func SendMessageV2(c *gin.Context) {
	fromId := c.PostForm("from_id")
	toId := c.PostForm("to_id")
	content := c.PostForm("content")
	cType := c.PostForm("type")
	if content == "" {
		c.JSON(200, gin.H{
			"code": 400,
			"msg":  "内容不能为空",
		})
		return
	}
	var kefuInfo models.User
	var vistorInfo models.Visitor
	if cType == "kefu" {
		kefuInfo = models.FindUser(fromId)
		vistorInfo = models.FindVisitorByVistorId(toId)
	} else if cType == "visitor" {
		vistorInfo = models.FindVisitorByVistorId(fromId)
		kefuInfo = models.FindUser(toId)
		// 已接入会话以数据库归属为准：访客页面缓存旧 to_id 时，消息仍应投递给当前接入客服。
		if vistorInfo.ID != 0 && vistorInfo.Claimed && vistorInfo.ToId != "" {
			if owner := models.FindUser(vistorInfo.ToId); owner.ID != 0 {
				kefuInfo = owner
			}
		}
	}

	if kefuInfo.ID == 0 || vistorInfo.ID == 0 {
		c.JSON(200, gin.H{
			"code": 400,
			"msg":  "用户不存在",
		})
		return
	}

	//限流：按会话身份计数，避免同一网络下客服与访客共用额度互相影响
	limitKey := "sendmessage:visitor:" + vistorInfo.VisitorId
	if cType == "kefu" {
		limitKey = "sendmessage:kefu:" + kefuInfo.Name
	}
	if !tools.LimitFreqSingle(limitKey, 10, 3) {
		c.JSON(200, gin.H{
			"code": 400,
			"msg":  "发送过于频繁，请稍后再试",
		})
		return
	}

	models.CreateMessage(kefuInfo.Name, vistorInfo.VisitorId, content, cType)
	//var msg TypeMessage
	if cType == "kefu" {
		guest, ok := ws.ClientList[vistorInfo.VisitorId]

		if guest != nil && ok {
			ws.VisitorMessage(vistorInfo.VisitorId, content, kefuInfo)
		}
		ws.KefuMessage(vistorInfo.VisitorId, content, kefuInfo)
		//msg = TypeMessage{
		//	Type: "message",
		//	Data: ws.ClientMessage{
		//		Name:    kefuInfo.Nickname,
		//		Avator:  kefuInfo.Avator,
		//		Id:      vistorInfo.VisitorId,
		//		Time:    time.Now().Format("2006-01-02 15:04:05"),
		//		ToId:    vistorInfo.VisitorId,
		//		Content: content,
		//		IsKefu:  "yes",
		//	},
		//}
		//str2, _ := json.Marshal(msg)
		//ws.OneKefuMessage(kefuInfo.Name, str2)
		c.JSON(200, gin.H{
			"code": 200,
			"msg":  "ok",
		})
	}
	if cType == "visitor" {
		guest, ok := ws.ClientList[vistorInfo.VisitorId]
		if ok && guest != nil {
			guest.UpdateTime = time.Now()
		}
		//kefuConns, ok := ws.KefuList[kefuInfo.Name]
		//if kefuConns == nil || !ok {
		//	c.JSON(200, gin.H{
		//		"code": 200,
		//		"msg":  "ok",
		//	})
		//	return
		//}
		msg := ws.TypeMessage{
			Type: "message",
			Data: ws.ClientMessage{
				Avator:  vistorInfo.Avator,
				Id:      vistorInfo.VisitorId,
				Name:    vistorInfo.Name,
				ToId:    kefuInfo.Name,
				Content: content,
				Time:    time.Now().Format("2006-01-02 15:04:05"),
				IsKefu:  "no",
			},
		}
		str, _ := json.Marshal(msg)
		ws.OneKefuMessage(kefuInfo.Name, str)
		//ws.KefuMessage(vistorInfo.VisitorId, content, kefuInfo)
		kefu, ok := ws.KefuList[kefuInfo.Name]
		if !ok || kefu == nil {
			go SendNoticeEmail(content+"|"+vistorInfo.Name, content)
		}
		go ws.VisitorAutoReply(vistorInfo, kefuInfo, content)
		go models.UpdateVisitorLastMessage(vistorInfo.VisitorId, content)
		c.JSON(200, gin.H{
			"code": 200,
			"msg":  "ok",
		})
	}

}

func SendKefuMessage(c *gin.Context) {
	fromId, _ := c.Get("kefu_name")
	toId := c.PostForm("to_id")
	content := c.PostForm("content")
	cType := c.PostForm("type")
	if content == "" {
		c.JSON(200, gin.H{
			"code": 400,
			"msg":  "内容不能为空",
		})
		return
	}
	//限流
	if !tools.LimitFreqSingle("sendmessage:kefu:"+fromId.(string), 10, 3) {
		c.JSON(200, gin.H{
			"code": 400,
			"msg":  "发送过于频繁，请稍后再试",
		})
		return
	}
	var kefuInfo models.User
	var vistorInfo models.Visitor
	kefuInfo = models.FindUser(fromId.(string))
	vistorInfo = models.FindVisitorByVistorId(toId)

	if kefuInfo.ID == 0 || vistorInfo.ID == 0 {
		c.JSON(200, gin.H{
			"code": 400,
			"msg":  "用户不存在",
		})
		return
	}
	if !vistorInfo.Claimed || vistorInfo.ToId != fromId.(string) {
		c.JSON(200, gin.H{
			"code": 409,
			"msg":  "请先接入该访客，已接入访客只能由归属客服回复",
		})
		return
	}

	models.CreateMessage(kefuInfo.Name, vistorInfo.VisitorId, content, cType)
	//var msg TypeMessage

	guest, ok := ws.ClientList[vistorInfo.VisitorId]

	if guest != nil && ok {
		ws.VisitorMessage(vistorInfo.VisitorId, content, kefuInfo)
	}
	ws.KefuMessage(vistorInfo.VisitorId, content, kefuInfo)
	go models.UpdateVisitorLastMessage(vistorInfo.VisitorId, content)
	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "ok",
	})

}
func SendVisitorNotice(c *gin.Context) {
	notice := c.Query("msg")
	if notice == "" {
		c.JSON(200, gin.H{
			"code": 400,
			"msg":  "msg不能为空",
		})
		return
	}
	msg := ws.TypeMessage{
		Type: "notice",
		Data: notice,
	}
	str, _ := json.Marshal(msg)
	for _, visitor := range ws.ClientList {
		visitor.Conn.WriteMessage(websocket.TextMessage, str)
	}
	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "ok",
	})
}
func SendCloseMessageV2(c *gin.Context) {
	visitorId := c.Query("visitor_id")
	if visitorId == "" {
		c.JSON(200, gin.H{
			"code": 400,
			"msg":  "visitor_id不能为空",
		})
		return
	}

	oldUser, ok := ws.ClientList[visitorId]
	if oldUser != nil || ok {
		msg := ws.TypeMessage{
			Type: "force_close",
			Data: visitorId,
		}
		str, _ := json.Marshal(msg)
		err := oldUser.Conn.WriteMessage(websocket.TextMessage, str)
		oldUser.Conn.Close()
		delete(ws.ClientList, visitorId)
		tools.Logger().Println("close_message", oldUser, err)
	}
	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "ok",
	})
}

const maxUploadSize = 90 * 1024 * 1024

func UploadImg(c *gin.Context) {
	f, err := c.FormFile("imgfile")
	if err != nil || f.Size <= 0 || f.Size >= maxUploadSize {
		c.JSON(200, gin.H{"code": 400, "msg": "上传失败!图片不能为空且必须小于90M"})
		return
	}
	fileExt := strings.ToLower(path.Ext(f.Filename))
	expectedTypes := map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp"}
	expectedType, allowed := expectedTypes[fileExt]
	if !allowed {
		c.JSON(200, gin.H{"code": 400, "msg": "上传失败!只允许png,jpg,gif,jpeg,webp图片"})
		return
	}
	src, err := f.Open()
	if err != nil {
		c.JSON(200, gin.H{"code": 400, "msg": "上传失败!无法读取图片"})
		return
	}
	defer src.Close()
	buffer := make([]byte, 512)
	n, readErr := src.Read(buffer)
	if readErr != nil && readErr != io.EOF {
		c.JSON(200, gin.H{"code": 400, "msg": "上传失败!无法读取图片"})
		return
	}
	if http.DetectContentType(buffer[:n]) != expectedType {
		c.JSON(200, gin.H{"code": 400, "msg": "上传失败!图片内容与扩展名不一致"})
		return
	}
	fildDir := fmt.Sprintf("%s%d%s/", common.Upload, time.Now().Year(), time.Now().Month().String())
	if err := os.MkdirAll(fildDir, 0755); err != nil {
		c.JSON(200, gin.H{"code": 500, "msg": "上传失败!无法创建目录"})
		return
	}
	filepath := fmt.Sprintf("%s%s%s", fildDir, tools.Uuid(), fileExt)
	if err := c.SaveUploadedFile(f, filepath); err != nil {
		c.JSON(200, gin.H{"code": 500, "msg": "上传失败!无法保存图片"})
		return
	}
	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "上传成功!",
		"result": gin.H{
			"path": filepath,
		},
	})
}
func UploadFile(c *gin.Context) {
	f, err := c.FormFile("realfile")
	if err != nil || f.Size <= 0 || f.Size >= maxUploadSize {
		c.JSON(200, gin.H{"code": 400, "msg": "上传失败!文件不能为空且必须小于90M"})
		return
	}
	fileExt := strings.ToLower(path.Ext(f.Filename))
	if len(fileExt) > 20 {
		c.JSON(200, gin.H{"code": 400, "msg": "上传失败!文件扩展名过长"})
		return
	}
	fildDir := fmt.Sprintf("%s%d%s/", common.Upload, time.Now().Year(), time.Now().Month().String())
	if err := os.MkdirAll(fildDir, 0755); err != nil {
		c.JSON(200, gin.H{"code": 500, "msg": "上传失败!无法创建目录"})
		return
	}
	filepath := fmt.Sprintf("%s%s%s", fildDir, tools.Uuid(), fileExt)
	if err := c.SaveUploadedFile(f, filepath); err != nil {
		c.JSON(200, gin.H{"code": 500, "msg": "上传失败!无法保存文件"})
		return
	}
	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "上传成功!",
		"result": gin.H{
			"path": filepath,
			"ext":  fileExt,
			"size": f.Size,
			"name": f.Filename,
		},
	})
}
func GetMessagesV2(c *gin.Context) {
	visitorId := c.Query("visitor_id")
	messages := models.FindMessageByVisitorId(visitorId)
	//result := make([]map[string]interface{}, 0)
	chatMessages := make([]ChatMessage, 0)
	var visitor models.Visitor
	var kefu models.User
	for _, message := range messages {
		//item := make(map[string]interface{})
		if visitor.Name == "" || kefu.Name == "" {
			kefu = models.FindUser(message.KefuId)
			visitor = models.FindVisitorByVistorId(message.VisitorId)
		}
		var chatMessage ChatMessage
		chatMessage.Time = message.CreatedAt.Format("2006-01-02 15:04:05")
		chatMessage.Content = message.Content
		chatMessage.MesType = message.MesType
		if message.MesType == "kefu" {
			chatMessage.Name = kefu.Nickname
			chatMessage.Avator = kefu.Avator
		} else {
			chatMessage.Name = visitor.Name
			chatMessage.Avator = visitor.Avator
		}
		chatMessages = append(chatMessages, chatMessage)
	}
	models.ReadMessageByVisitorId(visitorId)
	c.JSON(200, gin.H{
		"code":   200,
		"msg":    "ok",
		"result": chatMessages,
	})
}
func GetMessagespages(c *gin.Context) {
	visitorId := c.Query("visitor_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pagesize", "10"))
	if pageSize > 20 {
		pageSize = 20
	}
	count := models.CountMessage("visitor_id = ?", visitorId)
	list := models.FindMessageByPage(uint(page), uint(pageSize), "message.visitor_id = ?", visitorId)
	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "ok",
		"result": gin.H{
			"count":    count,
			"page":     page,
			"list":     list,
			"pagesize": pageSize,
		},
	})
}
