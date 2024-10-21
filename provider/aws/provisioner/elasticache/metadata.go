package elasticache

import (
	"github.com/convox/convox/pkg/options"
)

type MetaData struct {
	Required                    *bool   `json:"required"`
	Ignore                      *bool   `json:"ignore"`
	Immutable                   *bool   `json:"immutable"`
	UpdatesWithSomeInterruption *bool   `json:"updatesWithSomeInterruption"`
	Default                     *string `json:"default"`
}

func GetParametersMetaDataForReplicationGroupInstall() map[string]*MetaData {
	return map[string]*MetaData{
		ParamReplicationGroupId: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamAtRestEncryptionEnabled: {
			Immutable: options.Bool(true),
			Default:   options.String("false"),
		},
		ParamAuthToken:                {},
		ParamAutoMinorVersionUpgrade:  {},
		ParamAutomaticFailoverEnabled: {},
		ParamCacheNodeType: {
			Required: options.Bool(true),
		},
		ParamCacheSubnetGroupName: {
			// convox will create it if not provided
			Immutable: options.Bool(true),
		},
		ParamCacheParameterGroupName: {},
		ParamEngine: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamEngineVersion: {
			Required: options.Bool(true),
		},
		ParamNumCacheClusters: {},
		ParamPort: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamReplicationGroupDescription: {
			Required: options.Bool(true),
			Default:  options.String("convox managed replication group"),
		},
		ParamDeletionProtection:       {},
		ParamSecurityGroupIds:         {},
		ParamTransitEncryptionEnabled: {},
		ParamTransitEncryptionMode:    {},
		ParamSubnetIds:                {},
		ParamVPC:                      {},
	}
}

func GetParametersMetaDataForReplicationGroupUpdate() map[string]*MetaData {
	return map[string]*MetaData{
		ParamReplicationGroupId: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamAtRestEncryptionEnabled: {
			Immutable: options.Bool(true),
		},
		ParamAuthToken:                {},
		ParamAutoMinorVersionUpgrade:  {},
		ParamAutomaticFailoverEnabled: {},
		ParamCacheNodeType:            {},
		ParamCacheSubnetGroupName: {
			Immutable: options.Bool(true),
		},
		ParamCacheParameterGroupName: {},
		ParamEngine: {
			Immutable: options.Bool(true),
		},
		ParamEngineVersion:    {},
		ParamNumCacheClusters: {},
		ParamPort: {
			Immutable: options.Bool(true),
		},
		ParamReplicationGroupDescription: {
			Required: options.Bool(true),
		},
		ParamDeletionProtection:       {},
		ParamSecurityGroupIds:         {},
		ParamTransitEncryptionEnabled: {},
		ParamTransitEncryptionMode:    {},
		ParamSubnetIds:                {},
		ParamApplyImmediately: {
			Default: options.String("true"),
		},
	}
}

func GetParametersMetaDataForReplicationGroupImport() map[string]*MetaData {
	m := map[string]*MetaData{}
	for _, p := range ParametersNameList() {
		m[p] = &MetaData{}
	}

	m[ParamReplicationGroupId] = &MetaData{
		Required:  options.Bool(true),
		Immutable: options.Bool(true),
	}

	m[ParamEngine] = &MetaData{
		Required:  options.Bool(true),
		Immutable: options.Bool(true),
	}

	m[ParamPort] = &MetaData{
		Required: options.Bool(true),
	}

	return m
}

func GetParametersMetaDataForCacheClusterInstall() map[string]*MetaData {
	return map[string]*MetaData{
		ParamCacheClusterId: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamAutoMinorVersionUpgrade: {},
		ParamCacheNodeType: {
			Required: options.Bool(true),
		},
		ParamCacheSubnetGroupName: {
			// convox will create it if not provided
			Immutable: options.Bool(true),
		},
		ParamCacheParameterGroupName: {},
		ParamEngine: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamEngineVersion: {
			Required: options.Bool(true),
		},
		ParamNumCacheNodes: {},
		ParamPort: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamDeletionProtection:       {},
		ParamSecurityGroupIds:         {},
		ParamTransitEncryptionEnabled: {},
		ParamSubnetIds:                {},
		ParamVPC:                      {},
	}
}

func GetParametersMetaDataForCacheClusterUpdate() map[string]*MetaData {
	return map[string]*MetaData{
		ParamCacheClusterId: {
			Required:  options.Bool(true),
			Immutable: options.Bool(true),
		},
		ParamAutoMinorVersionUpgrade: {},
		ParamCacheNodeType:           {},
		ParamCacheSubnetGroupName: {
			Immutable: options.Bool(true),
		},
		ParamCacheParameterGroupName: {},
		ParamEngine: {
			Immutable: options.Bool(true),
		},
		ParamEngineVersion: {},
		ParamNumCacheNodes: {},
		ParamPort: {
			Immutable: options.Bool(true),
		},
		ParamDeletionProtection:       {},
		ParamSecurityGroupIds:         {},
		ParamTransitEncryptionEnabled: {},
		ParamSubnetIds:                {},
		ParamVPC:                      {},
	}
}

func GetParametersMetaDataForCacheClusterImport() map[string]*MetaData {
	m := map[string]*MetaData{}
	for _, p := range ParametersNameList() {
		m[p] = &MetaData{}
	}

	m[ParamCacheClusterId] = &MetaData{
		Required:  options.Bool(true),
		Immutable: options.Bool(true),
	}

	m[ParamEngine] = &MetaData{
		Required:  options.Bool(true),
		Immutable: options.Bool(true),
	}

	m[ParamPort] = &MetaData{
		Required: options.Bool(true),
	}

	return m
}
