package cluster

import (
	"github.com/filanov/bm-inventory/models"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	clusterStatusInsufficient = "insufficient"
	clusterStatusReady        = "ready"
	clusterStatusInstalling   = "installing"
	clusterStatusInstalled    = "installed"
	clusterStatusError        = "error"
)

const (
	statusInfoReady        = "Cluster ready to be installed"
	statusInfoInsufficient = "cluster is insufficient, exactly 3 known master hosts are needed for installation"
	statusInfoInstalling   = "Installation in progress"
	statusInfoInstalled    = "installed"
)

type UpdateReply struct {
	State     string
	IsChanged bool
}

type baseState struct {
	//TODO remove when res: https://github.com/golangci/golangci-lint/issues/537
	log logrus.FieldLogger //nolint:structcheck
	db  *gorm.DB           //nolint:structcheck
}

func updateState(state string, statusInfo string, c *models.Cluster, db *gorm.DB, log logrus.FieldLogger) (*UpdateReply, error) {
	updates := map[string]interface{}{"status": state, "status_info": statusInfo}
	dbReply := db.Model(&models.Cluster{}).Where("id = ? and status = ?",
		c.ID.String(), swag.StringValue(c.Status)).Updates(updates)
	if dbReply.Error != nil {
		return nil, errors.Wrapf(dbReply.Error, "failed to update cluster %s state from %s to %s",
			c.ID.String(), swag.StringValue(c.Status), state)
	}
	if dbReply.RowsAffected == 0 {
		return nil, errors.Errorf("failed to update cluster %s state from %s to %s, nothing have changed",
			c.ID.String(), swag.StringValue(c.Status), state)
	}
	log.Infof("updated cluster %s from state <%s> to state <%s>", c.ID.String(), swag.StringValue(c.Status), state)
	return &UpdateReply{
		State:     state,
		IsChanged: state != swag.StringValue(c.Status),
	}, nil
}

func getKnownMastersNodesIds(c *models.Cluster, db *gorm.DB) ([]*strfmt.UUID, error) {

	var cluster models.Cluster
	var masterNodesIds []*strfmt.UUID
	if err := db.Preload("Hosts").First(&cluster, "id = ?", c.ID).Error; err != nil {
		return nil, errors.Errorf("cluster %s not found", c.ID)
	}
	for _, host := range cluster.Hosts {
		if host.Role == "master" && swag.StringValue(host.Status) == "known" {
			masterNodesIds = append(masterNodesIds, host.ID)
		}
	}
	return masterNodesIds, nil
}

func mapMasterHostsByStatus(c *models.Cluster) map[string][]*models.Host {
	hostMap := make(map[string][]*models.Host)

	for _, host := range c.Hosts {
		if host.Role != "master" {
			continue
		}
		if _, ok := hostMap[swag.StringValue(host.Status)]; ok {
			hostMap[swag.StringValue(host.Status)] = append(hostMap[swag.StringValue(host.Status)], host)
		} else {
			hostMap[swag.StringValue(host.Status)] = []*models.Host{host}
		}
	}
	return hostMap
}
