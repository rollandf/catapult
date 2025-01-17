package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-chi/chi/middleware"

	log "github.com/sirupsen/logrus"

	node "github.com/PUMATeam/catapult/pkg/node"
	"github.com/PUMATeam/catapult/pkg/util"

	"github.com/PUMATeam/catapult/pkg/model"
	"github.com/PUMATeam/catapult/pkg/repositories"
	uuid "github.com/satori/go.uuid"
)

func NewHostsService(hr repositories.Hosts, logger *log.Logger, connManager *node.Connections) Hosts {
	return &hostsService{
		hostsRepository: hr,
		log:             logger,
		connManager:     connManager,
	}
}

type hostsService struct {
	hostsRepository repositories.Hosts
	log             *log.Logger
	connManager     *node.Connections
}

// InitalizeHosts initializes running hosts when the app starts.
// Currently only creats grpc connection, soon will run health checks
func (hs *hostsService) InitializeHosts(ctx context.Context) []error {
	errors := make([]error, 0, 0)
	hosts, err := hs.hostsRepository.ListHosts(ctx)

	if err != nil {
		errors = append(errors, fmt.Errorf("Couldn't fetch hosts from DB"))
		return errors
	}

	for _, host := range hosts {
		address := fmt.Sprintf("%s:%d", host.Address, host.Port)
		if host.Status == model.UP {
			hs.log.WithContext(ctx).
				WithFields(log.Fields{
					"host":    host.Name,
					"address": address,
				}).
				Info("Initializing host connection")
			_, err := hs.connManager.CreateConnection(host.ID, address)
			if err != nil {
				errors = append(errors, err)
			}
		}
	}

	return errors
}

func (hs *hostsService) HostByID(ctx context.Context, id uuid.UUID) (*model.Host, error) {
	return hs.hostsRepository.HostByID(ctx, id)
}

func (hs *hostsService) ListHosts(ctx context.Context) ([]model.Host, error) {
	return hs.hostsRepository.ListHosts(ctx)
}

func (hs *hostsService) updateHostStatus(ctx context.Context, host model.Host, status model.Status) error {
	host.Status = status
	return hs.hostsRepository.UpdateHost(ctx, host)
}

func (hs *hostsService) Validate(ctx context.Context, host NewHost) error {
	hs.log.WithContext(ctx).
		WithFields(log.Fields{
			"requestID": ctx.Value(middleware.RequestIDKey),
			"host":      host.Name,
		}).
		Info("Validating host")

	h, err := hs.hostsRepository.HostByAddress(ctx, host.Address)
	if err != nil {
		hs.log.Error(err)
		return err
	}
	if h.ID != uuid.Nil && h.Status != model.FAILED {
		hs.log.WithContext(ctx).
			WithFields(log.Fields{
				"requestID": ctx.Value(middleware.RequestIDKey),
				"host":      host.Name,
			}).Errorf("Host with address %s already exists", host.Address)
		return ErrAlreadyExists
	}
	h, err = hs.hostsRepository.HostByName(ctx, host.Name)
	if err != nil {
		log.Error(err)
		return err
	}
	if h.ID != uuid.Nil && h.Status != model.FAILED {
		hs.log.WithContext(ctx).
			WithFields(log.Fields{
				"requestID": ctx.Value(middleware.RequestIDKey),
				"host":      host.Name,
			}).Error("Host with this name already exists")

		return ErrAlreadyExists
	}

	return nil
}

func (hs *hostsService) AddHost(ctx context.Context, newHost *NewHost) (uuid.UUID, error) {
	host := model.Host{
		Name:    newHost.Name,
		Address: newHost.Address,
		Status:  model.DOWN,
		User:    newHost.User,
		// TODO: encrypt the password
		Password: newHost.Password,
		Port:     newHost.Port,
	}

	id, err := hs.hostsRepository.AddHost(ctx, host)
	if err != nil {
		return uuid.Nil, err
	}

	host.ID = id
	return id, err
}

// InstallHost installs prerequisits on the host
// TODO: leaving it as public to allow a user add a host
// without installing right away
func (hs *hostsService) InstallHost(ctx context.Context, h model.Host, localNodePath string) {
	hi := hostInstall{
		User:            h.User,
		FcVersion:       fcVersion,
		AnsiblePassword: h.Password,
		LocalNodePath:   localNodePath,
		NodePort:        fmt.Sprintf("%d", h.Port),
	}

	hs.UpdateHostStatus(ctx, h, model.INSTALLING)

	ac := util.NewAnsibleCommand(util.SetupHostPlaybook,
		h.User,
		h.Address,
		util.StructToMap(hi, strings.ToLower),
		hs.log)

	err := ac.ExecuteAnsible()
	if err != nil {
		hs.log.WithContext(ctx).
			WithFields(log.Fields{
				"requestID": ctx.Value(middleware.RequestIDKey),
				"host":      h.Name,
			}).Error("Error during host install: ", err)
		hs.UpdateHostStatus(ctx, h, model.FAILED)
		return
	}
	address := fmt.Sprintf("%s:%d", h.Address, h.Port)
	hs.log.WithContext(ctx).
		WithFields(log.Fields{
			"requestID": ctx.Value(middleware.RequestIDKey),
			"host":      h.Name,
			"address":   address,
		}).Info("Create grpc connection for host")

	_, err = hs.connManager.CreateConnection(h.ID, address)
	if err != nil {
		hs.log.WithContext(ctx).
			WithFields(log.Fields{
				"requestID": ctx.Value(middleware.RequestIDKey),
				"host":      h.Name,
			}).Error("Failed to create grpc connections, will be retried upon sending a request")

	}

	// TODO send a health check to the host before
	hs.UpdateHostStatus(ctx, h, model.UP)
}

func (hs *hostsService) UpdateHostStatus(ctx context.Context, host model.Host, status model.Status) error {
	hs.log.WithContext(ctx).
		WithFields(log.Fields{
			"requestID": ctx.Value(middleware.RequestIDKey),
			"host":      host.Name,
		}).Infof("Updating status of host to %d", status)

	err := hs.updateHostStatus(ctx, host, model.UP)
	if err != nil {
		hs.log.WithContext(ctx).
			WithFields(log.Fields{
				"requestID": ctx.Value(middleware.RequestIDKey),
				"host":      host.Name,
			}).Errorf("Failed to update status of host to %d", status)
		return err
	}

	return nil
}

func (hs *hostsService) GetConnManager(ctx context.Context) *node.Connections {
	return hs.connManager
}

type NewHost struct {
	Name          string `json:"name"`
	Address       string `json:"address"`
	User          string `json:"user"`
	Password      string `json:"password"`
	LocalNodePath string `json:"local_node_path"`
	ShouldInstall bool   `json:"-"`
	Port          int    `json:"port"`
}

type hostInstall struct {
	User            string `json:"ignore"`
	AnsiblePassword string `json:"ansible_ssh_pass"`
	FcVersion       string
	LocalNodePath   string `json:"local_node_path"`
	NodePort        string `json:"node_port"`
}

// TODO make it configurable
const fcVersion = "0.17.0"
