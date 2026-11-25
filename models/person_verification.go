package models

import "time"

type PersonRegistry struct {
	Model
	Name             string `gorm:"size:100;not null" json:"name"`
	CredentialType   string `gorm:"size:20;not null;unique_index:idx_person_credential" json:"credential_type"`
	CredentialHash   string `gorm:"size:64;not null;unique_index:idx_person_credential" json:"-"`
	CredentialMasked string `gorm:"size:80;not null" json:"credential_masked"`
	Status           string `gorm:"size:20;not null;index" json:"status"`
}

type PersonVerification struct {
	Model
	RegistryID       *uint  `gorm:"index" json:"registry_id"`
	Name             string `gorm:"size:100;not null" json:"name"`
	CredentialType   string `gorm:"size:20;not null" json:"credential_type"`
	CredentialMasked string `gorm:"size:80;not null" json:"credential_masked"`
	Reason           string `gorm:"size:50;not null" json:"reason"`
	Result           string `gorm:"size:30;not null;index" json:"result"`
	OperatorID       uint   `gorm:"not null" json:"operator_id"`
	OperatorName     string `gorm:"size:100;not null" json:"operator_name"`
}

func ListPersonRegistries() ([]PersonRegistry, error) {
	var registries []PersonRegistry
	err := DB.Order("updated_at desc").Find(&registries).Error
	return registries, err
}

func FindPersonRegistryByID(id uint) (PersonRegistry, error) {
	var registry PersonRegistry
	err := DB.Where("id = ?", id).First(&registry).Error
	return registry, err
}

func FindPersonRegistryByCredential(credentialType, credentialHash string) (PersonRegistry, error) {
	var registry PersonRegistry
	err := DB.Where("credential_type = ? AND credential_hash = ?", credentialType, credentialHash).First(&registry).Error
	return registry, err
}

func CreatePersonRegistry(registry *PersonRegistry) error {
	registry.UpdatedAt = time.Now()
	return DB.Create(registry).Error
}

func UpdatePersonRegistry(registry *PersonRegistry) error {
	return DB.Model(&PersonRegistry{}).Where("id = ?", registry.ID).Updates(map[string]interface{}{
		"name":              registry.Name,
		"credential_type":   registry.CredentialType,
		"credential_hash":   registry.CredentialHash,
		"credential_masked": registry.CredentialMasked,
		"status":            registry.Status,
		"updated_at":        time.Now(),
	}).Error
}

func CreatePersonVerification(verification *PersonVerification) error {
	verification.UpdatedAt = time.Now()
	return DB.Create(verification).Error
}

func ListPersonVerifications(limit int) ([]PersonVerification, error) {
	var verifications []PersonVerification
	query := DB.Order("created_at desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&verifications).Error
	return verifications, err
}
