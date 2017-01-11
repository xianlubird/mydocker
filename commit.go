package main


import (
	log "github.com/Sirupsen/logrus"
	"fmt"
	"github.com/xianlubird/mydocker/container"
	"os/exec"
)

func commitContainer(containerName, imageName string){
	mntURL := fmt.Sprintf(container.MntUrl, containerName)
	mntURL += "/"

	imageTar := container.RootUrl + "/" + imageName + ".tar"

	if _, err := exec.Command("tar", "-czf", imageTar, "-C", mntURL, ".").CombinedOutput(); err != nil {
		log.Errorf("Tar folder %s error %v", mntURL, err)
	}
}
