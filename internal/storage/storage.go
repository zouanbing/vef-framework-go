package storage

import (
	"fmt"

	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/internal/storage/filesystem"
	"github.com/ilxqx/vef-framework-go/internal/storage/memory"
	"github.com/ilxqx/vef-framework-go/internal/storage/minio"
	"github.com/ilxqx/vef-framework-go/storage"
)

func NewService(cfg *config.StorageConfig, appCfg *config.AppConfig) (storage.Service, error) {
	provider := cfg.Provider
	if provider == "" {
		provider = config.StorageMemory
	}

	switch provider {
	case config.StorageMinIO:
		return minio.New(cfg.MinIO, appCfg)
	case config.StorageMemory:
		return memory.New(), nil
	case config.StorageFilesystem:
		return filesystem.New(cfg.Filesystem)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedStorageProvider, cfg.Provider)
	}
}
