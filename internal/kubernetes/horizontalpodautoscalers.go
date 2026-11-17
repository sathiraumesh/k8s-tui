package kubernetes

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// What horizontal pod autoscalers have of their own; the rest is the
// generic calls (generic.go). An autoscaler sets a deployment's replicas
// by how busy its pods are; it needs metrics-server.

// Autoscale makes an autoscaler for the deployment, between min and max
// pods, aiming at cpuPercent of the pods' CPU requests, like `kubectl
// autoscale deployment NAME --min=MIN --max=MAX --cpu=PERCENT% -n
// NAMESPACE`. It is called as the deployment.
func (c *Client) Autoscale(ctx context.Context, namespace, deployment, min, max, cpuPercent string, dryRun bool) error {
	lo, err1 := strconv.Atoi(min)
	hi, err2 := strconv.Atoi(max)
	cpu, err3 := strconv.Atoi(strings.TrimSuffix(cpuPercent, "%"))
	switch {
	case err1 != nil || err2 != nil || lo < 1 || hi < lo:
		return fmt.Errorf("min %q, max %q: type whole numbers, 1 ≤ min ≤ max", min, max)
	case err3 != nil || cpu < 1:
		return fmt.Errorf("cpu %q: type a percent of the pods' CPU request, e.g. 80", cpuPercent)
	}
	if _, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, deployment, metav1.GetOptions{}); err != nil {
		return err
	}
	lo32, cpu32 := int32(lo), int32(cpu)
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: deployment, Namespace: namespace},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment},
			MinReplicas:    &lo32,
			MaxReplicas:    int32(hi),
			Metrics: []autoscalingv2.MetricSpec{{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name:   corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{Type: autoscalingv2.UtilizationMetricType, AverageUtilization: &cpu32},
				},
			}},
		},
	}
	_, err := c.clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Create(ctx, hpa, metav1.CreateOptions{DryRun: dryRunOption(dryRun)})
	return err
}
