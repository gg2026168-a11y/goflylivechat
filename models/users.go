package models

import (
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"time"
)

type User struct {
	Model
	Name     string `json:"name"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
	Avator   string `json:"avator"`
	RoleName string `json:"role_name" sql:"-"`
	RoleId   string `json:"role_id" sql:"-"`
}

func CreateUser(name string, password string, avator string, nickname string) uint {
	user := &User{
		Name:     name,
		Password: password,
		Avator:   avator,
		Nickname: nickname,
	}
	user.UpdatedAt = time.Now()
	DB.Create(user)
	return user.ID
}
func UpdateUser(name string, password string, avator string, nickname string) {
	user := &User{
		Avator:   avator,
		Nickname: nickname,
	}
	user.UpdatedAt = time.Now()
	if password != "" {
		user.Password = password
	}
	DB.Model(&User{}).Where("name = ?", name).Update(user)
}
func UpdateUserPass(name string, pass string) {
	user := &User{
		Password: pass,
	}
	user.UpdatedAt = time.Now()
	DB.Model(user).Where("name = ?", name).Update("Password", pass)
}
func UpdateUserAvator(name string, avator string) {
	user := &User{
		Avator: avator,
	}
	user.UpdatedAt = time.Now()
	DB.Model(user).Where("name = ?", name).Update("Avator", avator)
}
func FindUser(username string) User {
	var user User
	DB.Where("name = ?", username).First(&user)
	return user
}

// FindDefaultKefu 返回可接待访客的默认客服账号，优先 admin。
func FindDefaultKefu() User {
	if user := FindUser("admin"); user.ID != 0 {
		return user
	}
	var users []User
	DB.Order("id asc").Limit(1).Find(&users)
	if len(users) == 0 {
		return User{}
	}
	return users[0]
}
func FindUserById(id interface{}) User {
	var user User
	DB.Select("user.*,role.name role_name,role.id role_id").Joins("join user_role on user.id=user_role.user_id").Joins("join role on user_role.role_id=role.id").Where("user.id = ?", id).First(&user)
	return user
}
func DeleteUserById(id string) {
	DB.Where("id = ?", id).Delete(User{})
}
func FindUsers() []User {
	var users []User
	DB.Select("user.*,role.name role_name").Joins("left join user_role on user.id=user_role.user_id").Joins("left join role on user_role.role_id=role.id").Order("user.id desc").Find(&users)
	return users
}
func FindUserRole(query interface{}, id interface{}) User {
	var user User
	DB.Select(query).Where("user.id = ?", id).Joins("join user_role on user.id=user_role.user_id").Joins("join role on user_role.role_id=role.id").First(&user)
	return user
}
