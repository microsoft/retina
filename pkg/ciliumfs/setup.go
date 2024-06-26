package ciliumfs

import (
	"os"

	"go.uber.org/zap"
)

const ciliumDir = "/var/run/cilium"

func Setup(l *zap.Logger) {
	// Create /var/run/cilium directory.
	fp, err := os.Stat(ciliumDir)
	if err != nil {
		l.Warn("Failed to stat directory", zap.String("dir path", ciliumDir), zap.Error(err))
		if os.IsNotExist(err) {
			l.Info("Directory does not exist", zap.String("dir path", ciliumDir), zap.Error(err))
			// Path does not exist. Create it.
			err = os.MkdirAll("/var/run/cilium", 0o755) //nolint:gomnd // 0o755 is the permission mode.
			if err != nil {
				l.Error("Failed to create directory", zap.String("dir path", ciliumDir), zap.Error(err))
				l.Panic("Failed to create directory", zap.String("dir path", ciliumDir), zap.Error(err))
			}
		} else {
			// Some other error. Return.
			l.Panic("Failed to stat directory", zap.String("dir path", ciliumDir), zap.Error(err))
		}
	}
	l.Info("Created directory", zap.String("dir path", ciliumDir), zap.Any("file", fp))
}
