package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"certificate.stream/service/pkg/certificate/v1"
	"github.com/google/uuid"
)

type SinkFile struct {
	localDir string
}

func (sf *SinkFile) String() string {
	return fmt.Sprintf("FileDir=%s", sf.localDir)
}

func (sf *SinkFile) Init(ctx context.Context) error {
	localDir := os.Getenv("SINK_FILE_DIRECTORY")
	if localDir == "" {
		return fmt.Errorf("missing environment variable: SINK_FILE_DIRECTORY")
	}
	localDir = strings.TrimRight(localDir, "/")
	sf.localDir = localDir
	_, err := os.Stat(localDir)
	return err
}

func (sf *SinkFile) Put(ctx context.Context, batch *certificate.Batch) error {
	if batch != nil {
		now := time.Now()
		// set base path
		basePath := fmt.Sprintf("%s/year=%d/month=%02d/day=%02d",
			sf.localDir, now.Year(), int(now.Month()), now.Day())
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			os.MkdirAll(basePath, 0777)
		}
		// set file path
		filePath := fmt.Sprintf("%s/%s.json", basePath, uuid.NewString())

		logs, err := json.Marshal(batch.Logs)
		if err != nil {
			return err
		}
		err = os.WriteFile(filePath, logs, 0777)
		if err != nil {
			return fmt.Errorf("could not write to local file: %s: %v", filePath, err)
		}
	}
	return nil
}
