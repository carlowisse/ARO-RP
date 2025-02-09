package cluster

// Copyright (c) Microsoft Corporation.
// Licensed under the Apache License 2.0.

import (
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
)

const (
	azureFederatedTokenFileLocation = "/var/run/secrets/openshift/serviceaccount/token"

	ccoSecretNamespace           = "openshift-cloud-credential-operator"
	ccoSecretName                = "azure-credentials"
	ccoSecretFilename            = "azure-ad-pod-identity-webhook-config.yaml"
	authenticationConfigName     = "cluster"
	authenticationConfigFilename = "cluster-authentication-02-config.yaml"
)

func (m *manager) generateWorkloadIdentityResources() (map[string]kruntime.Object, error) {
	if !m.doc.OpenShiftCluster.UsesWorkloadIdentity() {
		return nil, fmt.Errorf("generateWorkloadIdentityResources called for a CSP cluster")
	}

	resources := map[string]kruntime.Object{}
	if platformWorkloadIdentitySecrets, err := m.generatePlatformWorkloadIdentitySecrets(); err != nil {
		return nil, err
	} else {
		for _, secret := range platformWorkloadIdentitySecrets {
			key := fmt.Sprintf("%s-%s-credentials.yaml", secret.ObjectMeta.Namespace, secret.ObjectMeta.Name)
			resources[key] = secret
		}
	}

	if cloudCredentialOperatorSecret, err := m.generateCloudCredentialOperatorSecret(); err != nil {
		return nil, err
	} else {
		resources[ccoSecretFilename] = cloudCredentialOperatorSecret
	}

	if authenticationConfig, err := m.generateAuthenticationConfig(); err != nil {
		return nil, err
	} else {
		resources[authenticationConfigFilename] = authenticationConfig
	}

	return resources, nil
}

func (m *manager) generatePlatformWorkloadIdentitySecrets() ([]*corev1.Secret, error) {
	subscriptionId := m.subscriptionDoc.ID
	tenantId := m.subscriptionDoc.Subscription.Properties.TenantID
	region := m.doc.OpenShiftCluster.Location

	roles := m.platformWorkloadIdentityRolesByVersion.GetPlatformWorkloadIdentityRolesByRoleName()

	secrets := []*corev1.Secret{}
	for _, identity := range m.doc.OpenShiftCluster.Properties.PlatformWorkloadIdentityProfile.PlatformWorkloadIdentities {
		if role, ok := roles[identity.OperatorName]; ok {
			secrets = append(secrets, &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.Identifier(),
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: role.SecretLocation.Namespace,
					Name:      role.SecretLocation.Name,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"azure_client_id":            identity.ClientID,
					"azure_subscription_id":      subscriptionId,
					"azure_tenant_id":            tenantId,
					"azure_region":               region,
					"azure_federated_token_file": azureFederatedTokenFileLocation,
				},
			})
		}
	}

	return secrets, nil
}

func (m *manager) generateCloudCredentialOperatorSecret() (*corev1.Secret, error) {
	tenantId := m.subscriptionDoc.Subscription.Properties.TenantID

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.Identifier(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ccoSecretNamespace,
			Name:      ccoSecretName,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"azure_tenant_id": tenantId,
		},
	}, nil
}

func (m *manager) generateAuthenticationConfig() (*configv1.Authentication, error) {
	oidcIssuer := m.doc.OpenShiftCluster.Properties.ClusterProfile.OIDCIssuer
	if oidcIssuer == nil {
		return nil, fmt.Errorf("oidcIssuer not present in clusterdoc")
	}

	return &configv1.Authentication{
		TypeMeta: metav1.TypeMeta{
			APIVersion: configv1.SchemeGroupVersion.Identifier(),
			Kind:       "Authentication",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: authenticationConfigName,
		},
		Spec: configv1.AuthenticationSpec{
			ServiceAccountIssuer: (string)(*oidcIssuer),
		},
	}, nil
}
