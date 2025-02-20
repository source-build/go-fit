package fit

import (
	"flag"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// NewReadInConfig read configuration file
func NewReadInConfig(file string, isUseParam ...bool) error {
	if len(isUseParam) > 0 && isUseParam[0] {
		pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
		pflag.Parse()
		if err := viper.BindPFlags(pflag.CommandLine); err != nil {
			return err
		}
	}

	viper.SetConfigFile(file)
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	return nil
}
