package azuredevops

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/microsoft/azure-devops-go-api/azuredevops/serviceendpoint"
	crud "github.com/microsoft/terraform-provider-azuredevops/azuredevops/crud/serviceendpoint"

	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/utils/converter"
)

func resourceServiceEndpointKubernetes() *schema.Resource {
	r := crud.GenBaseServiceEndpointResource(flattenServiceEndpointKubernetes, expandServiceEndpointKubernetes)
	r.Schema["apiserver_url"] = &schema.Schema{
		Type:        schema.TypeString,
		Required:    true,
		Description: "URL to Kubernete's API-Server",
	}
	r.Schema["authorization_type"] = &schema.Schema{
		Type:         schema.TypeString,
		Required:     true,
		Description:  "Type of credentials to use",
		ValidateFunc: validation.StringInSlice([]string{"AzureSubscription", "Kubeconfig", "ServiceAccount"}, false),
	}
	r.Schema["azure_subscription"] = &schema.Schema{
		Type:        schema.TypeSet,
		Optional:    true,
		Description: "'AzureSubscription'-type of configuration",
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"azure_environment": {
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "AzureCloud",
					Description: "type of azure cloud: AzureCloud",
				},
				"cluster_name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "name of aks-resource",
				},
				"subscription_id": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "id of azure subscription",
				},
				"subscription_name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "name of azure subscription",
				},
				"tenant_id": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "id of aad-tenant",
				},
				"resourcegroup_id": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "id of resourcegroup",
				},
				"namespace": {
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "default",
					Description: "accessed namespace",
				},
			},
		},
	}
	r.Schema["kubeconfig"] = &schema.Schema{
		Type:        schema.TypeSet,
		Optional:    true,
		Description: "'Kubeconfig'-type of configuration",
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"kube_config": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "Content of the kubeconfig file. The configuration information in your kubeconfig file allows Kubernetes clients to talk to your Kubernetes API servers. This file is used by kubectl and all supported Kubernetes clients.",
				},
				"cluster_context": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "Context of your cluster",
				},
				"accept_untrusted_certs": {
					Type:        schema.TypeBool,
					Optional:    true,
					Default:     true,
					Description: "Enable this if your authentication uses untrusted certificates",
				},
			},
		},
	}
	r.Schema["service_account"] = &schema.Schema{
		Type:        schema.TypeSet,
		Optional:    true,
		Description: "'ServiceAccount'-type of configuration",
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"service_account_secret_yaml": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "Content of the yaml file defining the service account secret.",
					ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
						v := val.(string)
						stringCheckList := [3]string{"data:", "token:", "ca.crt:"}
						for _, element := range stringCheckList {
							if !strings.Contains(v, element) {
								errs = append(errs, fmt.Errorf("The service acount secret yaml does not contain '%v' field. Make sure that its present and try again", element))
							}
						}
						return warns, errs
					},
				},
			},
		},
	}
	return r
}

// Convert internal Terraform data structure to an AzDO data structure
func expandServiceEndpointKubernetes(d *schema.ResourceData) (*serviceendpoint.ServiceEndpoint, *string, error) {
	serviceEndpoint, projectID := crud.DoBaseExpansion(d)
	serviceEndpoint.Type = converter.String("kubernetes")
	serviceEndpoint.Url = converter.String(d.Get("apiserver_url").(string))

	switch d.Get("authorization_type").(string) {
	case "AzureSubscription":
		configurationRaw := d.Get("azure_subscription").(*schema.Set).List()
		configuration := configurationRaw[0].(map[string]interface{})

		serviceEndpoint.Authorization = &serviceendpoint.EndpointAuthorization{
			Parameters: &map[string]string{
				"azureEnvironment": configuration["azure_environment"].(string),
				"azureTenantId":    configuration["tenant_id"].(string),
			},
			Scheme: converter.String("Kubernetes"),
		}

		clusterID := fmt.Sprintf("/subscriptions/%s/resourcegroups/%s/providers/Microsoft.ContainerService/managedClusters/%s", configuration["subscription_id"].(string), configuration["resourcegroup_id"].(string), configuration["cluster_name"].(string))
		serviceEndpoint.Data = &map[string]string{
			"authorizationType":     "AzureSubscription",
			"azureSubscriptionId":   configuration["subscription_id"].(string),
			"azureSubscriptionName": configuration["subscription_name"].(string),
			"clusterId":             clusterID,
			"namespace":             configuration["namespace"].(string),
		}
	case "Kubeconfig":
		configurationRaw := d.Get("kubeconfig").(*schema.Set).List()
		configuration := configurationRaw[0].(map[string]interface{})

		clusterContextInput := configuration["cluster_context"].(string)
		if clusterContextInput == "" {
			kubeConfigYAML := configuration["kube_config"].(string)
			var kubeConfigYAMLUnmarshalled map[string]interface{}
			err := yaml.Unmarshal([]byte(kubeConfigYAML), &kubeConfigYAMLUnmarshalled)
			if err != nil {
				panic(err)
			}
			clusterContextInputList := kubeConfigYAMLUnmarshalled["contexts"].([]interface{})[0].(map[interface{}]interface{})
			clusterContextInput = clusterContextInputList["name"].(string)
		}

		serviceEndpoint.Authorization = &serviceendpoint.EndpointAuthorization{
			Parameters: &map[string]string{
				"clusterContext": clusterContextInput,
				"kubeconfig":     configuration["kube_config"].(string),
			},
			Scheme: converter.String("Kubernetes"),
		}

		serviceEndpoint.Data = &map[string]string{
			"authorizationType":    "Kubeconfig",
			"acceptUntrustedCerts": fmt.Sprintf("%v", configuration["accept_untrusted_certs"].(bool)),
		}
	case "ServiceAccount":
		configurationRaw := d.Get("service_account").(*schema.Set).List()
		configuration := configurationRaw[0].(map[string]interface{})
		secretYAML := configuration["service_account_secret_yaml"].(string)
		var secretYAMLUnmarshalled map[string]interface{}

		err := yaml.Unmarshal([]byte(secretYAML), &secretYAMLUnmarshalled)
		if err != nil {
			errResult := fmt.Errorf("service_account_secret_yaml contains an invalid YAML: %s", err)
			return nil, nil, errResult
		}

		secretYAMLData := secretYAMLUnmarshalled["data"].(map[interface{}]interface{})
		apiToken := secretYAMLData["token"].(string)
		serviceAccountCertificate := secretYAMLData["ca.crt"].(string)

		serviceEndpoint.Authorization = &serviceendpoint.EndpointAuthorization{
			Parameters: &map[string]string{
				"apiToken":                  apiToken,
				"serviceAccountCertificate": serviceAccountCertificate,
			},
			Scheme: converter.String("Token"),
		}

		serviceEndpoint.Data = &map[string]string{
			"authorizationType": "ServiceAccount",
		}

	}

	return serviceEndpoint, projectID, nil
}

// Convert AzDO data structure to internal Terraform data structure
func flattenServiceEndpointKubernetes(d *schema.ResourceData, serviceEndpoint *serviceendpoint.ServiceEndpoint, projectID *string) {
	crud.DoBaseFlattening(d, serviceEndpoint, projectID)
	d.Set("authorization_type", (*serviceEndpoint.Data)["authorizationType"])

	switch (*serviceEndpoint.Data)["authorizationType"] {
	case "AzureSubscription":
		d.Set("azure_environment", (*serviceEndpoint.Authorization.Parameters)["azureEnvironment"])
		d.Set("tenant_id", (*serviceEndpoint.Authorization.Parameters)["azureTenantId"])
		d.Set("subscription_id", (*serviceEndpoint.Authorization.Parameters)["azureSubscriptionId"])
		d.Set("subscription_name", (*serviceEndpoint.Authorization.Parameters)["azureSubscriptionName"])
		d.Set("namespace", (*serviceEndpoint.Authorization.Parameters)["namespace"])
	case "Kubeconfig":
		d.Set("accept_untrusted_certs", (*serviceEndpoint.Authorization.Parameters)["acceptUntrustedCerts"])
		d.Set("kube_config", (*serviceEndpoint.Authorization.Parameters)["kubeconfig"])
	case "ServiceAccount":
		d.Set("api_token", (*serviceEndpoint.Authorization.Parameters)["apiToken"])
		d.Set("service_account_certificate", (*serviceEndpoint.Authorization.Parameters)["serviceAccountCertificate"])
	}
}
