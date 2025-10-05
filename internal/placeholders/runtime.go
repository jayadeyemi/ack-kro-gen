// Package placeholders/runtime implements Phase 1 of the placeholder transformation:
// replacing runtime literal values with sentinel tokens.
//
// This phase runs during chart rendering and converts concrete values from the
// ValuesSpec (like "ack-s3-controller", "us-west-2") into sentinel tokens
// (like "_NAME_", "_AWS_REGION_") that will later be converted to schema references.

package placeholders

import (
	"sort"
	"strings"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
)

// BuildRuntimeSentinels constructs a literal->sentinel map using ValuesSpec values.
func BuildRuntimeSentinels(gs config.ValuesSpec) map[string]string {
	replacements := map[string]string{}

	add := func(value, sentinel string) {
		v := strings.TrimSpace(value)
		if v == "" {
			return
		}
		replacements[v] = sentinel
	}

	controllerName := defaultControllerName(gs)
	namespace := defaultNamespace(gs)
	imageRepo := defaultImageRepository(gs)
	imageTag := defaultImageTag(gs)

	add(gs.ReleaseName, "_NAME_")
	add(controllerName, "_NAME_")
	add(controllerName, "_CONTROLLER_SERVICE_")
	add(namespace, "_NAMESPACE_")
	add(gs.Namespace, "_NAMESPACE_")
	add(gs.AWS.Region, "_AWS_REGION_")
	add(gs.AWS.EndpointURL, "_AWS_ENDPOINT_URL_")
	add(gs.AWS.Credentials.SecretKey, "_AWS_SECRET_KEY_")
	add(gs.AWS.Credentials.SecretName, "_AWS_SECRET_NAME_")
	add(gs.AWS.Credentials.Profile, "_AWS_PROFILE_")
	add(gs.Image.Repository, "_IMAGE_REPOSITORY_")
	add(imageRepo, "_IMAGE_REPOSITORY_")
	add(gs.Image.Tag, "_IMAGE_TAG_")
	add(imageTag, "_IMAGE_TAG_")
	add(gs.ServiceAccount.Name, "_SA_NAME_")
	add(controllerName, "_SA_NAME_")

	return replacements
}

// ApplyRuntimeSentinels replaces literal values with sentinel tokens using longest-first ordering.
func ApplyRuntimeSentinels(in string, replacements map[string]string) string {
	if len(replacements) == 0 {
		return in
	}

	keys := make([]string, 0, len(replacements))
	for k := range replacements {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

	out := in
	for _, key := range keys {
		out = strings.ReplaceAll(out, key, replacements[key])
	}
	return out
}
