package azuredevops

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

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
	r.Schema["configuration"] = &schema.Schema{
		Type:        schema.TypeSet,
		Required:    true,
		Description: "Configuration of service endpoint",
		MinItems:    1,
		MaxItems:    1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"authorization_type": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "Type of credentials: KubeConfig, ServiceAccount, AzureSubscription",
				},
				"parameters": {
					Type:     schema.TypeMap,
					Required: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
		},
	}
	return r
}

// Convert internal Terraform data structure to an AzDO data structure
func expandServiceEndpointKubernetes(d *schema.ResourceData) (*serviceendpoint.ServiceEndpoint, *string) {
	configurations := d.Get("configuration").(*schema.Set).List()
	configuration := configurations[0].(map[string]interface{})
	parameters := configuration["parameters"].(map[string]interface{})

	serviceEndpoint, projectID := crud.DoBaseExpansion(d)
	serviceEndpoint.Authorization = &serviceendpoint.EndpointAuthorization{
		Parameters: &map[string]string{
			"azureEnvironment": parameters["azure_environment"].(string),
			"azureTenantId":    parameters["tenant_id"].(string),
		},
		Scheme: converter.String("Kubernetes"),
	}

	clusterId := fmt.Sprintf("/subscriptions/%s/resourcegroups/%s/providers/Microsoft.ContainerService/managedClusters/%s", parameters["subscription_id"].(string), parameters["resourcegroup_id"].(string), parameters["aks_name"].(string))
	serviceEndpoint.Data = &map[string]string{
		"authorizationType":     configuration["authorization_type"].(string),
		"azureSubscriptionId":   parameters["subscription_id"].(string),
		"azureSubscriptionName": parameters["subscription_name"].(string),
		"clusterId":             clusterId,
		"namespace":             parameters["namespace"].(string),
	}
	serviceEndpoint.Type = converter.String("kubernetes")
	serviceEndpoint.Url = converter.String(d.Get("apiserver_url").(string))
	return serviceEndpoint, projectID
}

// Convert AzDO data structure to internal Terraform data structure
func flattenServiceEndpointKubernetes(d *schema.ResourceData, serviceEndpoint *serviceendpoint.ServiceEndpoint, projectID *string) {
	crud.DoBaseFlattening(d, serviceEndpoint, projectID)
	d.Set("azure_environment", (*serviceEndpoint.Authorization.Parameters)["azureEnvironment"])
	d.Set("tenant_id", (*serviceEndpoint.Authorization.Parameters)["azureTenantId"])
	d.Set("authorization_type", (*serviceEndpoint.Data)["authorizationType"])
	d.Set("subscription_id", (*serviceEndpoint.Authorization.Parameters)["azureSubscriptionId"])
	d.Set("subscription_name", (*serviceEndpoint.Authorization.Parameters)["azureSubscriptionName"])
	d.Set("namespace", (*serviceEndpoint.Authorization.Parameters)["namespace"])
}
