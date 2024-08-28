package ciliumfs

import (
	"os"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const ciliumDir = "/var/run/cilium"

func Setup(l *zap.Logger) error {
	// Create /var/run/cilium directory.
	fp, err := os.Stat(ciliumDir)
	if err != nil {
		l.Warn("Failed to stat directory", zap.String("dir path", ciliumDir), zap.Error(err))
		if os.IsNotExist(err) {
			l.Info("Directory does not exist", zap.String("dir path", ciliumDir), zap.Error(err))
			// Path does not exist. Create it.
			err = os.MkdirAll("/var/run/cilium", 0o755) //nolint:gomnd // 0o755 is the permission mode.
			if err != nil {
				return errors.Wrap(err, "failed to create Cilium directory")
			}
		} else {
			// Some other error. Return.
			return errors.Wrap(err, "failed to stat Cilium directory")
		}
	}
	l.Info("Created directory", zap.String("dir path", ciliumDir))
	return nil
}
