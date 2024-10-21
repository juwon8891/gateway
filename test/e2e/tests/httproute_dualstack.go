// Copyright Envoy Gateway Authors
// SPDX-License-Identifier: Apache-2.0
// The full text of the Apache license is available in the LICENSE file at
// the root of the repo.

//go:build e2e
// +build e2e

package tests

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/gateway-api/conformance/utils/kubernetes"
	"sigs.k8s.io/gateway-api/conformance/utils/roundtripper"
	"sigs.k8s.io/gateway-api/conformance/utils/suite"
)

func init() {
	ConformanceTests = append(ConformanceTests, HTTPRouteDualStackTest)
}

var HTTPRouteDualStackTest = suite.ConformanceTest{
	ShortName:   "HTTPRouteDualStack",
	Description: "Test HTTPRoute support for IPv6 only, dual-stack, and IPv4 only services",
	Manifests:   []string{"testdata/httproute-dualstack.yaml"},
	Test: func(t *testing.T, suite *suite.ConformanceTestSuite) {
		ipFamily := os.Getenv("IP_FAMILY")
		ns := "gateway-conformance-infra"
		gwNN := types.NamespacedName{Name: "dualstack-gateway", Namespace: ns}

		switch ipFamily {
		case "ipv6":
			runRouteTest(t, suite, ns, gwNN, "infra-backend-v1-httproute-ipv6", "/ipv6-only", "IPv6")
		case "dual":
			runRouteTest(t, suite, ns, gwNN, "infra-backend-v1-httproute-dualstack", "/dual-stack", "IPv4")
			runRouteTest(t, suite, ns, gwNN, "infra-backend-v1-httproute-dualstack", "/dual-stack", "IPv6")
		case "ipv4":
			runRouteTest(t, suite, ns, gwNN, "infra-backend-v1-httproute-ipv4", "/ipv4-only", "IPv4")
		default:
			t.Skip("Skipping HTTPRouteDualStack test as IP_FAMILY is not set")
		}
	},
}

func runRouteTest(t *testing.T, suite *suite.ConformanceTestSuite, ns string, gwNN types.NamespacedName, routeName, path, protocol string) {
	routeNN := types.NamespacedName{Name: routeName, Namespace: ns}
	gwAddr := kubernetes.GatewayAndHTTPRoutesMustBeAccepted(t, suite.Client, suite.TimeoutConfig, suite.ControllerName, kubernetes.NewGatewayRef(gwNN), routeNN)

	t.Run(fmt.Sprintf("%s Connectivity", protocol), func(t *testing.T) {
		if !checkConnectivity(t, suite, ns, gwAddr, path, protocol) {
			t.Fatalf("Failed to route to %s pod after retries", protocol)
		}
	})
}

func checkConnectivity(t *testing.T, suite *suite.ConformanceTestSuite, ns, gwAddr, path, protocol string) bool {
	// expectedResponse := http.ExpectedResponse{
	// 	Request:   http.Request{Path: path},
	// 	Response:  http.Response{StatusCode: 200},
	// 	Namespace: ns,
	// }

	gwURL, err := url.Parse(fmt.Sprintf("http://%s%s", gwAddr, path))
	if err != nil {
		t.Fatalf("Failed to parse gateway address: %v", err)
	}

	t.Logf("Attempting to connect to %s", gwURL.String())

	return wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		return validateResponse(t, suite, ns, gwAddr, gwURL, protocol)
	}) == nil
}

func validateResponse(t *testing.T, suite *suite.ConformanceTestSuite, ns, gwAddr string, gwURL *url.URL, protocol string) (bool, error) {
	// IPv6 주소인지 확인
	parsedIP := net.ParseIP(gwAddr)
	if protocol == "IPv6" && (parsedIP == nil || parsedIP.To4() != nil) {
		t.Fatalf("Expected an IPv6 address, but got: %s", gwAddr)
	}

	// IPv6 주소에 맞게 URL 형식 지정
	if protocol == "IPv6" {
		newURL, parseErr := url.Parse(fmt.Sprintf("http://[%s]%s", gwAddr, gwURL.Path))
		if parseErr != nil {
			t.Fatalf("Failed to parse gateway address as IPv6: %v", parseErr)
		}
		gwURL = newURL // 새로운 gwURL을 할당
	}

	capturedReq, capturedRes, err := suite.RoundTripper.CaptureRoundTrip(roundtripper.Request{
		URL:    *gwURL,
		Host:   gwAddr,
		Method: "GET",
		T:      t,
	})
	if err != nil {
		t.Logf("Failed to send request: %v. Retrying...", err)
		return false, nil
	}

	if capturedRes.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", capturedRes.StatusCode)
		return false, nil
	}

	matchingIP := findMatchingIP(t, suite, ns, capturedReq.Pod, protocol)
	if matchingIP == nil {
		t.Logf("No matching %s IP found for pod %s. Retrying...", protocol, capturedReq.Pod)
		return false, nil
	}

	t.Logf("Successfully routed to %s pod %s with IP: %s", protocol, capturedReq.Pod, matchingIP)
	return true, nil
}

func findMatchingIP(t *testing.T, suite *suite.ConformanceTestSuite, ns, podName, protocol string) net.IP {
	pod := &corev1.Pod{}
	if err := suite.Client.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: podName}, pod); err != nil {
		t.Fatalf("Failed to get pod %s: %v", podName, err)
	}

	for _, podIP := range pod.Status.PodIPs {
		parsedIP := net.ParseIP(podIP.IP)
		if parsedIP == nil {
			continue
		}
		if (protocol == "IPv4" && parsedIP.To4() != nil) || (protocol == "IPv6" && parsedIP.To4() == nil) {
			return parsedIP
		}
	}
	return nil
}
