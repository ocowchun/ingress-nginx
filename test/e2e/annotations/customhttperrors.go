/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package annotations

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	extensions "k8s.io/api/extensions/v1beta1"

	"k8s.io/ingress-nginx/test/e2e/framework"
)

var _ = framework.IngressNginxDescribe("Annotations - custom-http-errors", func() {
	f := framework.NewDefaultFramework("custom-http-errors")

	BeforeEach(func() {
		f.NewEchoDeploymentWithReplicas(1)
	})

	AfterEach(func() {
	})

	It("configures Nginx correctly", func() {
		host := "customerrors.foo.com"

		errorCodes := []string{"404", "500"}

		annotations := map[string]string{
			"nginx.ingress.kubernetes.io/custom-http-errors": strings.Join(errorCodes, ","),
		}

		ing := framework.NewSingleIngress(host, "/", host, f.IngressController.Namespace, "http-svc", 80, &annotations)
		f.EnsureIngress(ing)

		var serverConfig string
		f.WaitForNginxServer(host, func(sc string) bool {
			serverConfig = sc
			return strings.Contains(serverConfig, fmt.Sprintf("server_name %s", host))
		})

		By("turning on proxy_intercept_errors directive")
		Expect(serverConfig).Should(ContainSubstring("proxy_intercept_errors on;"))

		By("configuring error_page directive")
		for _, code := range errorCodes {
			Expect(serverConfig).Should(ContainSubstring(fmt.Sprintf("error_page %s = @custom_%s", code, code)))
		}

		By("creating error locations")
		for _, code := range errorCodes {
			Expect(serverConfig).Should(ContainSubstring(fmt.Sprintf("location @custom_%s", code)))
		}

		By("updating configuration when only custom-http-error value changes")
		err := framework.UpdateIngress(f.KubeClientSet, f.IngressController.Namespace, host, func(ingress *extensions.Ingress) error {
			ingress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/custom-http-errors"] = "503"
			return nil
		})
		Expect(err).ToNot(HaveOccurred())
		f.WaitForNginxServer(host, func(sc string) bool {
			if serverConfig != sc {
				serverConfig = sc
				return true
			}
			return false
		})
		Expect(serverConfig).Should(ContainSubstring("location @custom_503"))
		Expect(serverConfig).ShouldNot(ContainSubstring("location @custom_400"))
		Expect(serverConfig).ShouldNot(ContainSubstring("location @custom_500"))

		By("ignoring duplicate values (503 in this case) per server")
		annotations["nginx.ingress.kubernetes.io/custom-http-errors"] = "404, 503"
		ing = framework.NewSingleIngress(fmt.Sprintf("%s-else", host), "/else", host, f.IngressController.Namespace, "http-svc", 80, &annotations)
		f.EnsureIngress(ing)
		f.WaitForNginxServer(host, func(sc string) bool {
			serverConfig = sc
			return strings.Contains(serverConfig, "location /else")
		})
		count := strings.Count(serverConfig, fmt.Sprintf("location @custom_503"))
		Expect(count).Should(Equal(1))
	})
})
