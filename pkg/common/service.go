// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package common

import (
	"fmt"
	"net"
)

func NewRetinaSvc(name, namespace string, ips *IPAddresses, lbIP net.IP, selector map[string]string) *RetinaSvc {
	return &RetinaSvc{
		BaseObject: GetBaseObject(name, namespace, ips),
		lbIP:       lbIP,
		selector:   selector,
	}
}

func (s *RetinaSvc) DeepCopy() interface{} {
	s.RLock()
	defer s.RUnlock()

	newS := &RetinaSvc{
		BaseObject: s.BaseObject.DeepCopy(),
	}

	if s.lbIP != nil {
		newS.lbIP = make(net.IP, len(s.lbIP))
		copy(newS.lbIP, s.lbIP)
	}

	if s.selector != nil {
		newS.selector = make(map[string]string)
		for k, v := range s.selector {
			newS.selector[k] = v
		}
	}

	return newS
}

func (s *RetinaSvc) GetPrimaryIP() (string, error) {
	s.RLock()
	defer s.RUnlock()

	if s.ips != nil {
		pip := s.ips.PrimaryIP()
		if pip != "" {
			return pip, nil
		}
	}

	return "", fmt.Errorf("no primary IP found for service %s", s.Key())
}

func (s *RetinaSvc) LBIP() net.IP {
	s.RLock()
	defer s.RUnlock()

	return s.lbIP
}

func (s *RetinaSvc) SetLBIP(lbIP net.IP) {
	s.Lock()
	defer s.Unlock()
	s.lbIP = lbIP
}

func (s *RetinaSvc) Selector() map[string]string {
	s.RLock()
	defer s.RUnlock()

	return s.selector
}

func (s *RetinaSvc) SetSelector(selector map[string]string) {
	s.Lock()
	defer s.Unlock()
	s.selector = selector
}

func (s *RetinaSvc) SetIPs(ips *IPAddresses) {
	s.Lock()
	defer s.Unlock()
	s.ips = ips
}

func (s *RetinaSvc) IPs() *IPAddresses {
	s.RLock()
	defer s.RUnlock()

	return s.ips
}
