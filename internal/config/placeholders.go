package config

import "strings"

const (
	placeholderReleaseName    = "_NAME_"
	placeholderNamespace      = "_NAMESPACE_"
	placeholderAWSRegion      = "_AWS_REGION_"
	placeholderAWSAccountID   = "_AWS_ACCOUNT_ID_"
	placeholderAWSCredentials = "_AWS_SECRET_KEY_"
	placeholderAWSSecretName  = "_AWS_SECRET_NAME_"
	placeholderAWSProfile     = "_AWS_PROFILE_"
	placeholderImageRepo      = "_IMAGE_REPOSITORY_"
	placeholderImageTag       = "_IMAGE_TAG_"
	placeholderSAName         = "_SA_NAME_"
	placeholderIRSAArn        = "_IRSA_ARN_"
	placeholderLogLevel       = "_LOG_LEVEL_"
	placeholderLogDev         = "_LOG_DEV_"
	placeholderWatchNamespace = "_WATCH_NAMESPACE_"

	irsaAnnotationKey = "eks.amazonaws.com/role-arn"
)

// applyGraphDefaults ensures that optional GraphSpec fields are populated with the
// legacy placeholder values so callers only need to specify service+version.
func applyGraphDefaults(g *GraphSpec) {
	if g == nil {
		return
	}

	g.ReleaseName = fallbackPlaceholder(g.ReleaseName, placeholderReleaseName)
	g.Namespace = fallbackPlaceholder(g.Namespace, placeholderNamespace)

	g.AWS.Region = fallbackPlaceholder(g.AWS.Region, placeholderAWSRegion)
	g.AWS.AccountID = fallbackPlaceholder(g.AWS.AccountID, placeholderAWSAccountID)
	g.AWS.Credentials = fallbackPlaceholder(g.AWS.Credentials, placeholderAWSCredentials)
	g.AWS.SecretName = fallbackPlaceholder(g.AWS.SecretName, placeholderAWSSecretName)
	g.AWS.Profile = fallbackPlaceholder(g.AWS.Profile, placeholderAWSProfile)

	g.Image.Repository = fallbackPlaceholder(g.Image.Repository, placeholderImageRepo)
	g.Image.Tag = fallbackPlaceholder(g.Image.Tag, placeholderImageTag)

	g.ServiceAccount.Name = fallbackPlaceholder(g.ServiceAccount.Name, placeholderSAName)
	if g.ServiceAccount.Annotations == nil {
		g.ServiceAccount.Annotations = map[string]string{}
	}
	if fallbackPlaceholder(g.ServiceAccount.Annotations[irsaAnnotationKey], "") == "" {
		g.ServiceAccount.Annotations[irsaAnnotationKey] = placeholderIRSAArn
	}

	g.Controller.LogLevel = fallbackPlaceholder(g.Controller.LogLevel, placeholderLogLevel)
	g.Controller.LogDev = fallbackPlaceholder(g.Controller.LogDev, placeholderLogDev)
	g.Controller.WatchNamespace = fallbackPlaceholder(g.Controller.WatchNamespace, placeholderWatchNamespace)
}

func fallbackPlaceholder(v, placeholder string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return placeholder
	}
	return s
}
