package infra

import (
	"strings"
	"time"

	"github.com/alibaba/sealer/logger"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"

	v1 "github.com/alibaba/sealer/types/api/v1"
)

type ActionName string

const (
	CreateVPC           ActionName = "CreateVPC"
	CreateVSwitch       ActionName = "CreateVSwitch"
	CreateSecurityGroup ActionName = "CreateSecurityGroup"
	ReconcileInstance   ActionName = "ReconcileInstance"
	BindEIP             ActionName = "BindEIP"
	ReleaseEIP          ActionName = "ReleaseEIP"
	ClearInstances      ActionName = "ClearInstances"
	DeleteVSwitch       ActionName = "DeleteVSwitch"
	DeleteSecurityGroup ActionName = "DeleteSecurityGroup"
	DeleteVPC           ActionName = "DeleteVPC"
	GetZoneID           ActionName = "GetZoneID"
)

type AliProvider struct {
	Config    Config
	EcsClient ecs.Client
	VpcClient vpc.Client
	Cluster   *v1.Cluster
}

type Config struct {
	AccessKey    string
	AccessSecret string
	RegionID     string
}

type Alifunc func() error

const (
	Scheme                     = "https"
	IPProtocol                 = "tcp"
	APIServerPortRange         = "6443/6443"
	SSHPortRange               = "22/22"
	SourceCidrIP               = "0.0.0.0/0"
	CidrBlock                  = "172.16.0.0/24"
	Policy                     = "accept"
	DestinationResource        = "InstanceType"
	InstanceChargeType         = "PostPaid"
	ImageID                    = "centos_7_9_x64_20G_alibase_20210128.vhd"
	AccessKey                  = "ACCESSKEYID"
	AccessSecret               = "ACCESSKEYSECRET"
	Product                    = "product"
	Role                       = "role"
	Master                     = "master"
	Node                       = "node"
	Stopped                    = "Stopped"
	AvailableTypeStatus        = "WithStock"
	Bandwidth                  = "100"
	Digits                     = "0123456789"
	Specials                   = "~=+%^*/()[]{}/!@#$?|"
	Letter                     = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	PasswordLength             = 16
	DataCategory               = "cloud_ssd"
	AliDomain                  = "sea.aliyun.com/"
	AliCloud                   = "ALI_CLOUD"
	EipID                      = AliDomain + "EipID"
	Master0ID                  = AliDomain + "Master0ID"
	Master0InternalIP          = AliDomain + "Master0InternalIP"
	VpcID                      = AliDomain + "VpcID"
	VSwitchID                  = AliDomain + "VSwitchID"
	SecurityGroupID            = AliDomain + "SecurityGroupID"
	Eip                        = AliDomain + "ClusterEIP"
	ZoneID                     = AliDomain + "ZoneID"
	RegionID                   = "RegionID"
	AliRegionID                = AliDomain + RegionID
	AliMasterIDs               = AliDomain + "MasterIDs"
	AliNodeIDs                 = AliDomain + "NodeIDs"
	DefaultReigonID            = "cn-chengdu"
	AliCloudEssd               = "cloud_essd"
	TryTimes                   = 10
	TrySleepTime               = time.Second
	JustGetInstanceInfo        = ""
	ShouldBeDeleteInstancesIDs = "ShouldBeDeleteInstancesIDs"
)

var RecocileFuncMap = map[ActionName]func(provider *AliProvider) error{
	CreateVPC: func(aliProvider *AliProvider) error {
		return aliProvider.ReconcileResource(VpcID, aliProvider.CreateVPC)
	},

	CreateVSwitch: func(aliProvider *AliProvider) error {
		return aliProvider.ReconcileResource(VSwitchID, aliProvider.CreateVSwitch)
	},
	CreateSecurityGroup: func(aliProvider *AliProvider) error {
		return aliProvider.ReconcileResource(SecurityGroupID, aliProvider.CreateSecurityGroup)
	},
	ReconcileInstance: func(aliProvider *AliProvider) error {
		err := aliProvider.ReconcileIntances(Master)
		if err != nil {
			return err
		}

		err = aliProvider.ReconcileIntances(Node)
		if err != nil {
			return err
		}
		return nil
	},
	GetZoneID: func(aliProvider *AliProvider) error {
		return aliProvider.ReconcileResource(ZoneID, aliProvider.GetZoneID)
	},
	BindEIP: func(aliProvider *AliProvider) error {
		return aliProvider.ReconcileResource(EipID, aliProvider.BindEipForMaster0)
	},
}

var DeleteFuncMap = map[ActionName]func(provider *AliProvider){
	ReleaseEIP: func(aliProvider *AliProvider) {
		aliProvider.DeleteResource(EipID, aliProvider.ReleaseEipAddress)
	},
	ClearInstances: func(aliProvider *AliProvider) {
		var instanceIDs []string
		roles := []string{Master, Node}
		for _, role := range roles {
			instances, err := aliProvider.GetInstancesInfo(role, JustGetInstanceInfo)
			if err != nil {
				logger.Error("get %s instanceinfo failed %v", role, err)
			}
			for _, instance := range instances {
				instanceIDs = append(instanceIDs, instance.InstanceID)
			}
		}
		if len(instanceIDs) != 0 {
			aliProvider.Cluster.Annotations[ShouldBeDeleteInstancesIDs] = strings.Join(instanceIDs, ",")
		}
		aliProvider.DeleteResource(ShouldBeDeleteInstancesIDs, aliProvider.DeleteInstances)
	},
	DeleteVSwitch: func(aliProvider *AliProvider) {
		aliProvider.DeleteResource(VSwitchID, aliProvider.DeleteVSwitch)
	},
	DeleteSecurityGroup: func(aliProvider *AliProvider) {
		aliProvider.DeleteResource(SecurityGroupID, aliProvider.DeleteSecurityGroup)
	},
	DeleteVPC: func(aliProvider *AliProvider) {
		aliProvider.DeleteResource(VpcID, aliProvider.DeleteVPC)
	},
}

func (a *AliProvider) NewClient() error {
	ecsClient, err := ecs.NewClientWithAccessKey(a.Config.RegionID, a.Config.AccessKey, a.Config.AccessSecret)
	if err != nil {
		return err
	}
	vpcClient, err := vpc.NewClientWithAccessKey(a.Config.RegionID, a.Config.AccessKey, a.Config.AccessSecret)
	if err != nil {
		return err
	}
	a.EcsClient = *ecsClient
	a.VpcClient = *vpcClient
	return nil
}

func (a *AliProvider) ClearCluster() {
	todolist := []ActionName{
		ReleaseEIP,
		ClearInstances,
		DeleteVSwitch,
		DeleteSecurityGroup,
		DeleteVPC,
	}
	for _, name := range todolist {
		DeleteFuncMap[name](a)
	}
}

func (a *AliProvider) Reconcile() error {
	if a.Cluster.Annotations == nil {
		a.Cluster.Annotations = make(map[string]string)
	}
	if a.Cluster.DeletionTimestamp != nil {
		logger.Info("DeletionTimestamp not nil Clear Cluster")
		a.ClearCluster()
		return nil
	}
	if a.Cluster.Spec.SSH.Passwd == "" {
		// Create ssh password
		a.CreatePassword()
	}
	todolist := []ActionName{
		CreateVPC,
		GetZoneID,
		CreateVSwitch,
		CreateSecurityGroup,
		ReconcileInstance,
		BindEIP,
	}

	for _, actionname := range todolist {
		err := RecocileFuncMap[actionname](a)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *AliProvider) Apply() error {
	return a.Reconcile()
}
