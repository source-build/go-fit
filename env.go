package fit

import (
	"log"
	"os"
)

type EnvType string

const (
	EnvNil         EnvType = ""
	EnvDevelopment EnvType = "development"
	EnvProduction  EnvType = "production"
)

// GetProjectEnv obtaining project environment variables,
// if present but with values other than development or production, will cause panic.
func GetProjectEnv(name string) EnvType {
	env := os.Getenv(name)
	if env == "" {
		log.Println("Unable to find an environment variable named '" + env + "', which represents the development environment of the project. Optional values: development or production. Will use 'development' as the default value.")
		env = "development"
	} else if env != "development" && env != "production" {
		panic("Environment variable " + env + " can only select the following optional values:development or production")
	}
	return EnvType(env)
}
