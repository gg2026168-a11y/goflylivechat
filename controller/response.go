package controller

var (
	Port string
)

type Response struct {
	Code   int         `json:"code"`
	Msg    string      `json:"msg"`
	result interface{} `json:"result"`
}
type ChatMessage struct {
	Time    string `json:"time"`
	Content string `json:"content"`
	MesType string `json:"mes_type"`
	Name    string `json:"name"`
	Avator  string `json:"avator"`
}
type VisitorOnline struct {
	Uid         string `json:"uid"`
	Username    string `json:"username"`
	Avator      string `json:"avator"`
	LastMessage string `json:"last_message"`
	Claimed     bool   `json:"claimed"`
	OwnerId     string `json:"owner_id"`
	OwnerName   string `json:"owner_name"`
	OwnerAvatar string `json:"owner_avatar"`
}
type GetuiResponse struct {
	Code float64                `json:"code"`
	Msg  string                 `json:"msg"`
	Data map[string]interface{} `json:"data"`
}
type VisitorExtra struct {
	VisitorName   string `json:"visitorName"`
	VisitorAvatar string `json:"visitorAvatar"`
}
