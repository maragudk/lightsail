package lightsail

import (
	"context"
	"errors"
	"log"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lightsail"
	"github.com/aws/aws-sdk-go-v2/service/lightsail/types"
)

type Lightsail struct {
	client *lightsail.Client
	log    *log.Logger
}

type NewOptions struct {
	Config aws.Config
	Log    *log.Logger
}

func New(opts NewOptions) *Lightsail {
	return &Lightsail{
		client: lightsail.NewFromConfig(opts.Config),
		log:    opts.Log,
	}
}

var containerNameMatcher = regexp.MustCompile(`^:[a-z]+\.([a-z]+)\.\d+$`)

func (l *Lightsail) Deploy(service string) error {
	log.Println("Deploying", service)

	// Get all current deployments and find the active one
	ctx, cancel := newContext()
	defer cancel()
	deployments, err := l.client.GetContainerServiceDeployments(ctx, &lightsail.GetContainerServiceDeploymentsInput{ServiceName: &service})
	if err != nil {
		return err
	}
	if len(deployments.Deployments) == 0 {
		return errors.New("no deployments for service, create one first")
	}

	var activeDeployment types.ContainerServiceDeployment
	for _, d := range deployments.Deployments {
		if d.State != types.ContainerServiceDeploymentStateActive {
			continue
		}
		activeDeployment = d
		break
	}
	log.Println("Active deployment is version", *activeDeployment.Version)

	// Get all current container images and find the newest one for each container in the active deployment
	ctx, cancel = newContext()
	defer cancel()
	newestContainerImage := map[string]string{}
	images, err := l.client.GetContainerImages(ctx, &lightsail.GetContainerImagesInput{ServiceName: &service})
	if err != nil {
		return err
	}
	for _, i := range images.ContainerImages {
		name := containerNameMatcher.ReplaceAllString(*i.Image, "$1")
		if _, ok := newestContainerImage[name]; !ok {
			newestContainerImage[name] = *i.Image
		}
		if len(newestContainerImage) == len(activeDeployment.Containers) {
			break
		}
	}
	log.Println("New container images:", newestContainerImage)

	// Use the same container configurations as previously, but with the newest image
	containers := map[string]types.Container{}
	for name, c := range activeDeployment.Containers {
		image := newestContainerImage[name]
		containers[name] = types.Container{
			Command:     c.Command,
			Environment: c.Environment,
			Image:       &image,
			Ports:       c.Ports,
		}
	}

	// Create the deployment with the new container configuration and the same public endpoint
	ctx, cancel = newContext()
	defer cancel()
	deploy, err := l.client.CreateContainerServiceDeployment(ctx, &lightsail.CreateContainerServiceDeploymentInput{
		ServiceName: &service,
		Containers:  containers,
		PublicEndpoint: &types.EndpointRequest{
			ContainerName: activeDeployment.PublicEndpoint.ContainerName,
			ContainerPort: activeDeployment.PublicEndpoint.ContainerPort,
			HealthCheck:   activeDeployment.PublicEndpoint.HealthCheck,
		},
	})
	if err != nil {
		return err
	}
	nextVersion := deploy.ContainerService.NextDeployment.Version
	log.Println("Deploying version", *nextVersion)

	for {
		ctx, cancel = newContext()
		deployments, err = l.client.GetContainerServiceDeployments(ctx, &lightsail.GetContainerServiceDeploymentsInput{ServiceName: &service})
		if err != nil {
			cancel()
			return err
		}
		cancel()

		var state types.ContainerServiceDeploymentState
		for _, d := range deployments.Deployments {
			if *d.Version == *nextVersion {
				state = d.State
				break
			}
		}
		switch state {
		case types.ContainerServiceDeploymentStateActivating:
			time.Sleep(time.Second)
			continue
		case types.ContainerServiceDeploymentStateActive:
			log.Println("Deployed", service)
			return nil
		case types.ContainerServiceDeploymentStateFailed:
			log.Println("Deploying", service, "failed")
			// TODO
			return errors.New("blerp")
		}
	}
}

func newContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
