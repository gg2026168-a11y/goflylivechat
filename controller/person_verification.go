package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"goflylivechat/models"
)

const (
	personStatusActive   = "active"
	personStatusDisabled = "disabled"
)

var (
	personCredentialTypes = map[string]bool{
		"id_card":  true,
		"passport": true,
		"mobile":   true,
		"other":    true,
	}
	personVerificationReasons = map[string]bool{
		"活动入场核验": true,
		"会员身份确认": true,
		"会议人员审核": true,
	}
	personIDCardPattern   = regexp.MustCompile(`^\d{17}[\dX]$`)
	personPassportPattern = regexp.MustCompile(`^[A-Z0-9]{5,20}$`)
	personOtherPattern    = regexp.MustCompile(`^[A-Z0-9-]{4,64}$`)
	personIDCardWeights   = []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
)

type personRegistryRequest struct {
	Name             string `json:"name"`
	CredentialType   string `json:"credential_type"`
	CredentialNumber string `json:"credential_number"`
	Status           string `json:"status"`
}

type personVerificationRequest struct {
	Name             string `json:"name"`
	CredentialType   string `json:"credential_type"`
	CredentialNumber string `json:"credential_number"`
	Reason           string `json:"reason"`
}

func writePersonError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"code": status, "msg": message})
}

func normalizePersonName(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func normalizePersonCredential(credentialType, raw string) (string, string) {
	value := strings.TrimSpace(raw)
	switch credentialType {
	case "id_card":
		value = strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(value))
		if !personIDCardPattern.MatchString(value) || !validPersonIDCardChecksum(value) {
			return "", "请输入正确的 18 位居民身份证号码"
		}
	case "passport":
		value = strings.ToUpper(value)
		if !personPassportPattern.MatchString(value) {
			return "", "护照号码应为 5-20 位字母或数字"
		}
	case "mobile":
		value = normalizeChinaMobile(value)
		if !chinaMobilePattern.MatchString(value) {
			return "", "请输入正确的中国大陆 11 位手机号"
		}
	case "other":
		value = strings.ToUpper(value)
		if !personOtherPattern.MatchString(value) {
			return "", "其他证件号码应为 4-64 位字母、数字或连字符"
		}
	default:
		return "", "请选择有效的证件类型"
	}
	return value, ""
}

func validPersonIDCardChecksum(value string) bool {
	if len(value) != 18 {
		return false
	}
	sum := 0
	for index, weight := range personIDCardWeights {
		sum += int(value[index]-'0') * weight
	}
	return "10X98765432"[sum%11] == value[17]
}

func normalizePersonRegistryInput(request personRegistryRequest, credentialRequired bool) (personRegistryRequest, string) {
	request.Name = normalizePersonName(request.Name)
	request.CredentialType = strings.TrimSpace(request.CredentialType)
	request.Status = strings.TrimSpace(request.Status)
	if length := utf8.RuneCountInString(request.Name); length < 2 || length > 50 {
		return request, "人员姓名应为 2-50 个字符"
	}
	if !personCredentialTypes[request.CredentialType] {
		return request, "请选择有效的证件类型"
	}
	if request.Status != personStatusActive && request.Status != personStatusDisabled {
		return request, "请选择有效的档案状态"
	}
	if strings.TrimSpace(request.CredentialNumber) == "" {
		if credentialRequired {
			return request, "请输入证件号码"
		}
		return request, ""
	}
	normalized, message := normalizePersonCredential(request.CredentialType, request.CredentialNumber)
	if message != "" {
		return request, message
	}
	request.CredentialNumber = normalized
	return request, ""
}

func personRegistryHMACKey() string {
	return strings.TrimSpace(os.Getenv("PERSON_REGISTRY_HMAC_KEY"))
}

func personCredentialHash(credentialType, credentialNumber, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(credentialType))
	mac.Write([]byte{0})
	mac.Write([]byte(credentialNumber))
	return hex.EncodeToString(mac.Sum(nil))
}

func maskPersonCredential(credentialType, value string) string {
	if credentialType == "id_card" && len(value) == 18 {
		return value[:6] + "********" + value[14:]
	}
	if credentialType == "mobile" && len(value) == 11 {
		return value[:3] + "****" + value[7:]
	}
	if len(value) <= 4 {
		return "****"
	}
	if len(value) <= 8 {
		return value[:1] + "***" + value[len(value)-1:]
	}
	return value[:3] + "****" + value[len(value)-3:]
}

func personOperator(c *gin.Context) (uint, string) {
	idValue, _ := c.Get("kefu_id")
	id, _ := idValue.(float64)
	nameValue, _ := c.Get("kefu_name")
	name, _ := nameValue.(string)
	return uint(id), strings.TrimSpace(name)
}

func GetPersonRegistries(c *gin.Context) {
	registries, err := models.ListPersonRegistries()
	if err != nil {
		writePersonError(c, http.StatusInternalServerError, "人员档案读取失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "result": registries})
}

func PostPersonRegistry(c *gin.Context) {
	var request personRegistryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writePersonError(c, http.StatusBadRequest, "档案信息格式无效")
		return
	}
	if strings.TrimSpace(request.Status) == "" {
		request.Status = personStatusActive
	}
	request, message := normalizePersonRegistryInput(request, true)
	if message != "" {
		writePersonError(c, http.StatusBadRequest, message)
		return
	}
	key := personRegistryHMACKey()
	if key == "" {
		writePersonError(c, http.StatusServiceUnavailable, "人员档案服务尚未配置 PERSON_REGISTRY_HMAC_KEY")
		return
	}
	credentialHash := personCredentialHash(request.CredentialType, request.CredentialNumber, key)
	if _, err := models.FindPersonRegistryByCredential(request.CredentialType, credentialHash); err == nil {
		writePersonError(c, http.StatusConflict, "该证件档案已存在")
		return
	} else if !gorm.IsRecordNotFoundError(err) {
		writePersonError(c, http.StatusInternalServerError, "人员档案查重失败")
		return
	}
	registry := models.PersonRegistry{
		Name:             request.Name,
		CredentialType:   request.CredentialType,
		CredentialHash:   credentialHash,
		CredentialMasked: maskPersonCredential(request.CredentialType, request.CredentialNumber),
		Status:           request.Status,
	}
	if err := models.CreatePersonRegistry(&registry); err != nil {
		writePersonError(c, http.StatusInternalServerError, "人员档案保存失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "人员档案已保存", "result": registry})
}

func PutPersonRegistry(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		writePersonError(c, http.StatusBadRequest, "档案编号无效")
		return
	}
	var request personRegistryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writePersonError(c, http.StatusBadRequest, "档案信息格式无效")
		return
	}
	registry, err := models.FindPersonRegistryByID(uint(id))
	if gorm.IsRecordNotFoundError(err) {
		writePersonError(c, http.StatusNotFound, "人员档案不存在")
		return
	}
	if err != nil {
		writePersonError(c, http.StatusInternalServerError, "人员档案读取失败")
		return
	}
	if strings.TrimSpace(request.Status) == "" {
		request.Status = registry.Status
	}
	request, message := normalizePersonRegistryInput(request, false)
	if message != "" {
		writePersonError(c, http.StatusBadRequest, message)
		return
	}
	if request.CredentialNumber == "" {
		if request.CredentialType != registry.CredentialType {
			writePersonError(c, http.StatusBadRequest, "变更证件类型时请重新输入证件号码")
			return
		}
	} else {
		key := personRegistryHMACKey()
		if key == "" {
			writePersonError(c, http.StatusServiceUnavailable, "人员档案服务尚未配置 PERSON_REGISTRY_HMAC_KEY")
			return
		}
		credentialHash := personCredentialHash(request.CredentialType, request.CredentialNumber, key)
		existing, findErr := models.FindPersonRegistryByCredential(request.CredentialType, credentialHash)
		if findErr == nil && existing.ID != registry.ID {
			writePersonError(c, http.StatusConflict, "该证件档案已存在")
			return
		}
		if findErr != nil && !gorm.IsRecordNotFoundError(findErr) {
			writePersonError(c, http.StatusInternalServerError, "人员档案查重失败")
			return
		}
		registry.CredentialHash = credentialHash
		registry.CredentialMasked = maskPersonCredential(request.CredentialType, request.CredentialNumber)
	}
	registry.Name = request.Name
	registry.CredentialType = request.CredentialType
	registry.Status = request.Status
	if err := models.UpdatePersonRegistry(&registry); err != nil {
		writePersonError(c, http.StatusInternalServerError, "人员档案保存失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "人员档案已更新", "result": registry})
}

func PostPersonVerification(c *gin.Context) {
	var request personVerificationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writePersonError(c, http.StatusBadRequest, "查验信息格式无效")
		return
	}
	registryInput, message := normalizePersonRegistryInput(personRegistryRequest{
		Name:             request.Name,
		CredentialType:   request.CredentialType,
		CredentialNumber: request.CredentialNumber,
		Status:           personStatusActive,
	}, true)
	if message != "" {
		writePersonError(c, http.StatusBadRequest, message)
		return
	}
	request.Name = registryInput.Name
	request.CredentialType = registryInput.CredentialType
	request.CredentialNumber = registryInput.CredentialNumber
	request.Reason = strings.TrimSpace(request.Reason)
	if !personVerificationReasons[request.Reason] {
		writePersonError(c, http.StatusBadRequest, "请选择有效的查验事由")
		return
	}
	key := personRegistryHMACKey()
	if key == "" {
		writePersonError(c, http.StatusServiceUnavailable, "人员查验服务尚未配置 PERSON_REGISTRY_HMAC_KEY")
		return
	}
	credentialHash := personCredentialHash(request.CredentialType, request.CredentialNumber, key)
	registry, err := models.FindPersonRegistryByCredential(request.CredentialType, credentialHash)
	result := "not_found"
	var registryID *uint
	if err == nil {
		registryID = &registry.ID
		result = "matched"
		if registry.Name != request.Name || registry.Status != personStatusActive {
			result = "review_required"
		}
	} else if !gorm.IsRecordNotFoundError(err) {
		writePersonError(c, http.StatusInternalServerError, "人员档案查询失败")
		return
	}
	operatorID, operatorName := personOperator(c)
	verification := models.PersonVerification{
		RegistryID:       registryID,
		Name:             request.Name,
		CredentialType:   request.CredentialType,
		CredentialMasked: maskPersonCredential(request.CredentialType, request.CredentialNumber),
		Reason:           request.Reason,
		Result:           result,
		OperatorID:       operatorID,
		OperatorName:     operatorName,
	}
	if err := models.CreatePersonVerification(&verification); err != nil {
		writePersonError(c, http.StatusInternalServerError, "查验记录保存失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "查验完成", "result": verification})
}

func GetPersonVerificationRecords(c *gin.Context) {
	records, err := models.ListPersonVerifications(20)
	if err != nil {
		writePersonError(c, http.StatusInternalServerError, "查验记录读取失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "result": records})
}

func personCredentialTypeLabel(value string) string {
	labels := map[string]string{"id_card": "居民身份证", "passport": "护照", "mobile": "手机号码", "other": "其他证件"}
	if label, ok := labels[value]; ok {
		return label
	}
	return value
}

func personVerificationResultLabel(value string) string {
	labels := map[string]string{"matched": "内部档案一致", "not_found": "未找到内部档案", "review_required": "待人工复核"}
	if label, ok := labels[value]; ok {
		return label
	}
	return value
}

func personCSVCell(value string) string {
	if strings.HasPrefix(value, "=") || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "@") || strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") {
		return "'" + value
	}
	return value
}

func GetPersonVerificationExport(c *gin.Context) {
	records, err := models.ListPersonVerifications(0)
	if err != nil {
		writePersonError(c, http.StatusInternalServerError, "查验记录读取失败")
		return
	}
	c.Header("Content-Disposition", `attachment; filename="person-verifications.csv"`)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	_, _ = c.Writer.Write([]byte{0xef, 0xbb, 0xbf})
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"人员姓名", "证件类型", "脱敏证件号", "查验事由", "查验结果", "查验时间", "操作人"})
	for _, record := range records {
		_ = writer.Write([]string{
			personCSVCell(record.Name),
			personCredentialTypeLabel(record.CredentialType),
			record.CredentialMasked,
			personCSVCell(record.Reason),
			personVerificationResultLabel(record.Result),
			record.CreatedAt.Format("2006-01-02 15:04:05"),
			personCSVCell(record.OperatorName),
		})
	}
	writer.Flush()
}
