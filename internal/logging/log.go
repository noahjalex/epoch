package logging

import (
	"github.com/sirupsen/logrus"
)

func Init() *logrus.Logger {
	return logrus.New()
}
