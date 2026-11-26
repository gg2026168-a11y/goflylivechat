package controller

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"goflylivechat/common"
	"goflylivechat/models"
	"goflylivechat/tools"
	"goflylivechat/ws"
	"strconv"
)

//	func PostVisitor(c *gin.Context) {
//		name := c.PostForm("name")
//		avator := c.PostForm("avator")
//		toId := c.PostForm("to_id")
//		id := c.PostForm("id")
//		refer := c.PostForm("refer")
//		city := c.PostForm("city")
//		client_ip := c.PostForm("client_ip")
//		if name == "" || avator == "" || toId == "" || id == "" || refer == "" || city == "" || client_ip == "" {
//			c.JSON(200, gin.H{
//				"code": 400,
//				"msg":  "error",
//			})
//			return
//		}
//		kefuInfo := models.FindUser(toId)
//		if kefuInfo.ID == 0 {
//			c.JSON(200, gin.H{
//				"code": 400,
//				"msg":  "用户不存在",
//			})
//			return
//		}
//		models.CreateVisitor(name, avator, c.ClientIP(), toId, id, refer, city, client_ip)
//
//		userInfo := make(map[string]string)
//		userInfo["uid"] = id
//		userInfo["username"] = name
//		userInfo["avator"] = avator
//		msg := TypeMessage{
//			Type: "userOnline",
//			Data: userInfo,
//		}
//		str, _ := json.Marshal(msg)
//		kefuConns := kefuList[toId]
//		if kefuConns != nil {
//			for k, kefuConn := range kefuConns {
//				log.Println(k, "xxxxxxxx")
//				kefuConn.WriteMessage(websocket.TextMessage, str)
//			}
//		}
//		c.JSON(200, gin.H{
//			"code": 200,
//			"msg":  "ok",
//		})
//	}
func PostVisitorLogin(c *gin.Context) {

	avator := ""
	userAgent := c.GetHeader("User-Agent")
	if tools.IsMobile(userAgent) {
		avator = "/static/images/1.png"
	} else {
		avator = "/static/images/2.png"
	}

	toId := c.PostForm("to_id")
	id := c.PostForm("visitor_id")

	if id == "" {
		id = tools.Uuid()
	}
	refer := c.PostForm("refer")
	name := "Guest"
	city := ""
	countryname, cityname := tools.GetCity("./config/GeoLite2-City.mmdb", c.ClientIP())
	if countryname != "" || cityname != "" {
		city = fmt.Sprintf("%s %s", countryname, cityname)
		name = fmt.Sprintf("%s Guest", city)
	}

	client_ip := c.ClientIP()
	extra := c.PostForm("extra")
	extraJson := tools.Base64Decode(extra)
	if extraJson != "" {
		var extraObj VisitorExtra
		err := json.Unmarshal([]byte(extraJson), &extraObj)
		if err == nil {
			if extraObj.VisitorName != "" {
				name = extraObj.VisitorName
			}
			if extraObj.VisitorAvatar != "" {
				avator = extraObj.VisitorAvatar
			}
		}
	}
	//log.Println(name,avator,c.ClientIP(),toId,id,refer,city,client_ip)
	if name == "" || avator == "" || toId == "" || id == "" || refer == "" || client_ip == "" {
		c.JSON(200, gin.H{
			"code": 400,
			"msg":  "error",
		})
		return
	}
	kefuInfo := models.FindUser(toId)
	if kefuInfo.ID == 0 {
		c.JSON(200, gin.H{
			"code": 400,
			"msg":  "The customer service account does not exist",
		})
		return
	}
	visitor := models.FindVisitorByVistorId(id)
	if visitor.Name != "" {
		avator = visitor.Avator
		if visitor.Claimed {
			toId = visitor.ToId
		}
		//更新状态上线
		models.UpdateVisitor(name, visitor.Avator, id, 1, c.ClientIP(), c.ClientIP(), refer, extra, toId)
	} else {
		models.CreateVisitor(name, avator, c.ClientIP(), toId, id, refer, city, client_ip, extra)
	}
	visitor.Name = name
	visitor.Avator = avator
	visitor.ToId = toId
	visitor.ClientIp = c.ClientIP()
	visitor.VisitorId = id

	//各种通知
	go SendNoticeEmail(visitor.Name, " incoming!")
	//go SendAppGetuiPush(kefuInfo.Name, visitor.Name, visitor.Name+" incoming!")
	go SendVisitorLoginNotice(kefuInfo.Name, visitor.Name, visitor.Avator, visitor.Name+" incoming!", visitor.VisitorId)
	go ws.VisitorOnline(kefuInfo.Name, visitor)
	//go SendServerJiang(visitor.Name, "来了", c.Request.Host)

	c.JSON(200, gin.H{
		"code":   200,
		"msg":    "ok",
		"result": visitor,
	})
}
func GetVisitor(c *gin.Context) {
	visitorId := c.Query("visitorId")
	vistor := models.FindVisitorByVistorId(visitorId)
	c.JSON(200, gin.H{
		"code":   200,
		"msg":    "ok",
		"result": vistor,
	})
}

// @Summary 获取访客列表接口
// @Produce  json
// @Accept multipart/form-data
// @Param page query   string true "分页"
// @Param token header string true "认证token"
// @Success 200 {object} controller.Response
// @Failure 200 {object} controller.Response
// @Router /visitors [get]
func GetVisitors(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	pagesize, _ := strconv.Atoi(c.Query("pagesize"))
	if pagesize == 0 {
		pagesize = 10
	}
	kefuId, _ := c.Get("kefu_name")
	vistors := models.FindVisitorsByKefuId(uint(page), uint(pagesize), kefuId.(string))
	count := models.CountVisitorsByKefuId(kefuId.(string))
	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "ok",
		"result": gin.H{
			"list":     vistors,
			"count":    count,
			"pagesize": common.PageSize,
		},
	})
}

// @Summary 获取访客聊天信息接口
// @Produce  json
// @Accept multipart/form-data
// @Param visitorId query   string true "访客ID"
// @Param token header string true "认证token"
// @Success 200 {object} controller.Response
// @Failure 200 {object} controller.Response
// @Router /messages [get]
func GetVisitorMessage(c *gin.Context) {
	visitorId := c.Query("visitorId")

	query := "message.visitor_id= ?"
	messages := models.FindMessageByWhere(query, visitorId)
	result := make([]map[string]interface{}, 0)
	for _, message := range messages {
		item := make(map[string]interface{})

		item["time"] = message.CreatedAt.Format("2006-01-02 15:04:05")
		item["content"] = message.Content
		item["mes_type"] = message.MesType
		item["visitor_name"] = message.VisitorName
		item["visitor_avator"] = message.VisitorAvator
		item["kefu_name"] = message.KefuName
		item["kefu_avator"] = message.KefuAvator
		result = append(result, item)

	}
	go models.ReadMessageByVisitorId(visitorId)
	c.JSON(200, gin.H{
		"code":   200,
		"msg":    "ok",
		"result": result,
	})
}

// @Summary 获取在线访客列表接口
// @Produce  json
// @Success 200 {object} controller.Response
// @Failure 200 {object} controller.Response
// @Router /visitors_online [get]
func GetVisitorOnlines(c *gin.Context) {
	users := make([]map[string]string, 0)
	visitorIds := make([]string, 0)
	for uid, visitor := range ws.ClientList {
		userInfo := make(map[string]string)
		userInfo["uid"] = uid
		userInfo["name"] = visitor.Name
		userInfo["avator"] = visitor.Avator
		users = append(users, userInfo)
		visitorIds = append(visitorIds, visitor.Id)
	}

	//查询最新消息
	messages := models.FindLastMessage(visitorIds)
	temp := make(map[string]string, 0)
	for _, mes := range messages {
		temp[mes.VisitorId] = mes.Content
	}
	for _, user := range users {
		user["last_message"] = temp[user["uid"]]
	}

	tcps := make([]string, 0)
	for ip, _ := range clientTcpList {
		tcps = append(tcps, ip)
	}
	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "ok",
		"result": gin.H{
			"ws":  users,
			"tcp": tcps,
		},
	})
}

// @Summary 获取所有在线访客列表，供每位在线客服显示
// @Produce  json
// @Success 200 {object} controller.Response
// @Failure 200 {object} controller.Response
// @Router /visitors_kefu_online [get]
func GetKefusVisitorOnlines(c *gin.Context) {
	users := make([]*VisitorOnline, 0)
	visitorIds := make([]string, 0)
	for uid, visitor := range ws.ClientList {
		userInfo := new(VisitorOnline)
		userInfo.Uid = uid
		userInfo.Username = visitor.Name
		userInfo.Avator = visitor.Avator
		userInfo.Claimed = visitor.Claimed
		if visitor.Claimed {
			owner := models.FindUser(visitor.To_id)
			userInfo.OwnerId = visitor.To_id
			userInfo.OwnerName = owner.Nickname
			if userInfo.OwnerName == "" {
				userInfo.OwnerName = visitor.To_id
			}
			userInfo.OwnerAvatar = owner.Avator
		}
		users = append(users, userInfo)
		visitorIds = append(visitorIds, visitor.Id)
	}

	//查询最新消息
	messages := models.FindLastMessage(visitorIds)
	temp := make(map[string]string, 0)
	for _, mes := range messages {
		temp[mes.VisitorId] = mes.Content
	}
	for _, user := range users {
		user.LastMessage = temp[user.Uid]
		if user.LastMessage == "" {
			user.LastMessage = "new visitor"
		}
	}

	tcps := make([]string, 0)
	for ip, _ := range clientTcpList {
		tcps = append(tcps, ip)
	}
	c.JSON(200, gin.H{
		"code":   200,
		"msg":    "ok",
		"result": users,
	})
}

func visitorClaimResult(visitor models.Visitor) gin.H {
	owner := models.FindUser(visitor.ToId)
	ownerName := owner.Nickname
	if ownerName == "" {
		ownerName = visitor.ToId
	}
	return gin.H{
		"uid":          visitor.VisitorId,
		"visitor_id":   visitor.VisitorId,
		"claimed":      visitor.Claimed,
		"owner_id":     visitor.ToId,
		"owner_name":   ownerName,
		"owner_avatar": owner.Avator,
	}
}

// PostVisitorClaim 将未接入访客原子分配给当前客服，并同步给所有工作台。
func PostVisitorClaim(c *gin.Context) {
	visitorId := c.PostForm("visitor_id")
	if visitorId == "" {
		visitorId = c.Query("visitor_id")
	}
	if visitorId == "" {
		c.JSON(200, gin.H{"code": 400, "msg": "visitor_id不能为空"})
		return
	}
	kefuName, _ := c.Get("kefu_name")
	kefuId := kefuName.(string)
	kefuInfo := models.FindUser(kefuId)
	visitor := models.FindVisitorByVistorId(visitorId)
	if kefuInfo.ID == 0 || visitor.ID == 0 {
		c.JSON(200, gin.H{"code": 400, "msg": "访客或客服不存在"})
		return
	}
	if visitor.Claimed {
		if visitor.ToId != kefuId {
			c.JSON(200, gin.H{
				"code":   409,
				"msg":    "该访客已由其他客服接入",
				"result": visitorClaimResult(visitor),
			})
			return
		}
		c.JSON(200, gin.H{"code": 200, "msg": "接入成功", "result": visitorClaimResult(visitor)})
		return
	}
	if !models.ClaimVisitor(visitorId, kefuId) {
		latest := models.FindVisitorByVistorId(visitorId)
		if latest.Claimed {
			c.JSON(200, gin.H{
				"code":   409,
				"msg":    "该访客已由其他客服接入",
				"result": visitorClaimResult(latest),
			})
			return
		}
		c.JSON(200, gin.H{"code": 500, "msg": "接入失败，请稍后重试"})
		return
	}
	visitor = models.FindVisitorByVistorId(visitorId)
	ws.ClaimVisitorUser(visitorId, kefuId)
	ws.VisitorClaimed(visitor, kefuInfo)
	c.JSON(200, gin.H{"code": 200, "msg": "接入成功", "result": visitorClaimResult(visitor)})
}
