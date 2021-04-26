//+build mage

package main

import (
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Runs go mod download and then installs the binary.
func Build() error {
	if err := sh.Run("go", "mod", "download"); err != nil {
		return err
	}
	return nil
}

// Build a docker image for amd64
func BuildDockerAMD() error {
	if err := sh.RunV("docker", "build", "-t", "slimbean/adsb-loki:latest", "-f", "cmd/adsb-loki/Dockerfile", "."); err != nil {
		return err
	}
	return nil
}

// Build a docker image for arm32
func ARM32Image() error {
	if err := sh.RunV("docker", "build", "--build-arg", "TARGET_PLATFORM=linux/arm/v7", "--build-arg", "COMPILE_GOARCH=arm", "--build-arg", "COMPILE_GOARM=7", "-t", "slimbean/adsb-loki:latest", "-f", "cmd/adsb-loki/Dockerfile", "."); err != nil {
		return err
	}
	return nil
}

func ARM32Push() error {
	mg.Deps(ARM32Image)
	if err := sh.RunV("docker", "push", "slimbean/adsb-loki:latest"); err != nil {
		return err
	}
	return nil
}
