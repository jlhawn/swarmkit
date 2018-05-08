package service

import (
	"fmt"

	"github.com/docker/swarmkit/api"
)

func getServiceReplicasTxt(s *api.Service, running int) string {
	switch t := s.Spec.GetMode().(type) {
	case *api.ServiceSpec_Global:
		return "global"
	case *api.ServiceSpec_Replicated:
		return fmt.Sprintf("%d/%d", running, t.Replicated.Replicas)
	}
	return ""
}

func getServiceStaticMessageTxt(s *api.Service) string {
	if s.StaticInfo == nil {
		return ""
	}

	return s.StaticInfo.Message
}

func getServiceStaticAddressTxt(s *api.Service) string {
	if s.StaticInfo == nil || len(s.StaticInfo.NetworkAttachment.Addresses) == 0 {
		return ""
	}

	return s.StaticInfo.NetworkAttachment.Addresses[0]
}

func getServiceStaticGroupTxt(s *api.Service) string {
	staticMode := s.Spec.GetStatic()
	if staticMode == nil {
		return ""
	}

	return staticMode.PeerGroup
}

func getServiceStaticNodeTxt(s *api.Service) string {
	if s.StaticInfo == nil {
		return ""
	}

	return s.StaticInfo.NodeID
}
