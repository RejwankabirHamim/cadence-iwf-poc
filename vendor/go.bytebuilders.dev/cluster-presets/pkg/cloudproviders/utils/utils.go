/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Community License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	capicatalog "go.klusters.dev/capi-ops-manager/apis/catalog/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetCAPIVersionInfo(ctx context.Context, kc client.Client, k8sVersion string) (*capicatalog.CapiVersion, error) {
	curVersion, err := semver.NewVersion(k8sVersion)
	if err != nil {
		return nil, err
	}
	versionName := fmt.Sprintf("%d.%d", curVersion.Major(), curVersion.Minor())

	capiVersion := &capicatalog.CapiVersion{}
	err = kc.Get(ctx, types.NamespacedName{Name: versionName}, capiVersion)
	if err != nil {
		return nil, err
	}
	return capiVersion, nil
}

func CheckUpgradeable(from string, to string) (bool, error) {
	fromVersion, err := semver.NewVersion(from)
	if err != nil {
		return false, err
	}
	toVersion, err := semver.NewVersion(to)
	if err != nil {
		return false, err
	}
	if toVersion.Major()-fromVersion.Major() == 1 {
		return true, nil
	}
	if toVersion.Major() == fromVersion.Major() {
		return toVersion.GreaterThan(fromVersion), nil
	}
	return false, nil
}
