package syncers

import (
	"context"
	"fmt"

	"github.com/flanksource/vcluster-sync-host-secrets/constants"

	"github.com/loft-sh/vcluster-sdk/syncer"
	syncercontext "github.com/loft-sh/vcluster-sdk/syncer/context"
	"github.com/loft-sh/vcluster-sdk/translate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ManagedHostSecret = "plugin.vcluster.loft.sh/managed-by"
)

func NewSecretSyncer(ctx *syncercontext.RegisterContext, destinationNamespace string) syncer.Syncer {
	return &secretSyncer{
		hostNamespace:        ctx.TargetNamespace,
		DestinationNamespace: destinationNamespace,
	}
}

type secretSyncer struct {
	hostNamespace        string
	DestinationNamespace string
}

func (s *secretSyncer) Name() string {
	return constants.PluginName
}

func (s *secretSyncer) Resource() client.Object {
	return &corev1.Secret{}
}

// Make sure the interface is implemented
var _ syncer.Starter = &secretSyncer{}

// ReconcileStart is executed before the syncer or fake syncer reconcile starts and can
// return true if the rest of the reconcile should be skipped. If an error is returned,
// the reconcile will fail and try to requeue.
func (s *secretSyncer) ReconcileStart(ctx *syncercontext.SyncContext, req ctrl.Request) (bool, error) {
	// reconcile can be skipped if the Secret that triggered this reconciliation request
	// is not from the DestinationNamespace
	return req.Namespace != s.DestinationNamespace, nil
}

func (s *secretSyncer) ReconcileEnd() {
	// NOOP
}

// SyncUp will synchronise physical secrets into the vcluster
func (s *secretSyncer) SyncUp(ctx *syncercontext.SyncContext, pObj client.Object) (ctrl.Result, error) {
	pSecret := pObj.(*corev1.Secret)
	if pSecret.GetAnnotations()[constants.SyncAnnotation] != "true" {
		// Only sync selected secrets from the host cluster
		return ctrl.Result{}, nil
	}
	if pSecret.GetLabels()[translate.MarkerLabel] != "" {
		// Ignore secrets synced to the host by the vcluster
		return ctrl.Result{}, nil
	}
	labels := map[string]string{
		ManagedHostSecret: constants.PluginName,
	}
	for k, v := range pSecret.GetLabels() {
		labels[k] = v
	}
	namespace := s.DestinationNamespace
	if pSecret.GetAnnotations()[constants.NamespaceAnnotation] != "" {
		namespace = pSecret.GetAnnotations()[constants.NamespaceAnnotation]
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        pObj.GetName(),
			Annotations: pObj.GetAnnotations(),
			Labels:      labels,
		},
		Immutable:  pSecret.Immutable,
		Data:       pSecret.Data,
		Type:       pSecret.Type,
		StringData: pSecret.StringData,
	}

	err := ctx.VirtualClient.Create(context.Background(), secret)
	if err == nil {
		ctx.Log.Infof("created secret %s/%s", secret.GetNamespace(), secret.GetName())
	} else {
		err = fmt.Errorf("failed to create secret %s/%s: %v", secret.GetNamespace(), secret.GetName(), err)
	}
	return ctrl.Result{}, err
}

// Sync defines the action that should be taken by the syncer if a virtual cluster object
// and physical cluster object exist and either one of them has changed. The syncer is
// expected to reconcile in this case without knowledge of which object has actually
// changed. This is needed to avoid race conditions and defining a clear hierarchy what
// fields should be managed by which cluster. For example, for pods you would want to sync
// down (virtual -> physical) spec changes, while you would want to sync up
// (physical -> virtual) status changes, as those would get set only by the physical host
// cluster.
func (s *secretSyncer) Sync(ctx *syncercontext.SyncContext, pObj client.Object, vObj client.Object) (ctrl.Result, error) {
	pSecret := pObj.(*corev1.Secret)
	if pSecret.GetAnnotations()[constants.SyncAnnotation] != "true" {
		if vObj.GetLabels()[ManagedHostSecret] == constants.PluginName {
			// delete synced secret if the host secret no longer has the correct annotation
			err := ctx.VirtualClient.Delete(ctx.Context, vObj)
			if err == nil {
				ctx.Log.Infof("deleted secret %s/%s because host secret is no longer is annotated as %s: true", vObj.GetNamespace(), vObj.GetName(), constants.SyncAnnotation)
			} else {
				err = fmt.Errorf("failed to delete secret %s/%s: %v", vObj.GetNamespace(), vObj.GetName(), err)
			}
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	vSecret := vObj.(*corev1.Secret)
	updated := s.translateUpdateUp(pSecret, vSecret)
	if updated == nil {
		// No updated needed
		return ctrl.Result{}, nil
	}

	err := ctx.VirtualClient.Update(ctx.Context, updated)
	if err == nil {
		ctx.Log.Infof("updated secret %s/%s", vObj.GetNamespace(), vObj.GetName())
	} else {
		err = fmt.Errorf("failed to update secret %s/%s: %v", vObj.GetNamespace(), vObj.GetName(), err)
	}
	return ctrl.Result{}, err
}

// SyncDown is called when the secret in the host gets removed
// or if the vObj is an unrelated Secret created in vcluster
func (s *secretSyncer) SyncDown(ctx *syncercontext.SyncContext, vObj client.Object) (ctrl.Result, error) {
	if vObj.GetLabels()[ManagedHostSecret] == constants.PluginName {
		// Delete synced secret because the host secret was deleted
		err := ctx.VirtualClient.Delete(ctx.Context, vObj)
		if err == nil {
			ctx.Log.Infof("deleted secret %s/%s because host secret no longer exists", vObj.GetNamespace(), vObj.GetName())
		} else {
			err = fmt.Errorf("failed to delete secret %s/%s: %v", vObj.GetNamespace(), vObj.GetName(), err)
		}
		return ctrl.Result{}, err
	}
	// Ignore all unrelated secrets
	return ctrl.Result{}, nil

}

// IsManaged determines if a physical object is managed by the vcluster
func (s *secretSyncer) IsManaged(pObj client.Object) (bool, error) {
	// We will consider all Secrets as managed in order to reconcile
	// when a secret type changes, and we will check the annotations
	// in the Sync and SyncUp methods and ignore the irrelevant ones
	return true, nil
}

// VirtualToPhysical translates a virtual name to a physical name
func (s *secretSyncer) VirtualToPhysical(req types.NamespacedName, vObj client.Object) types.NamespacedName {
	// The secret that is being mirrored by a particular vObj secret
	// is located in the hostNamespace of the host cluster
	return types.NamespacedName{
		Namespace: s.hostNamespace,
		Name:      req.Name,
	}
}

// PhysicalToVirtual translates a physical name to a virtual name
func (s *secretSyncer) PhysicalToVirtual(pObj client.Object) types.NamespacedName {
	// The secret mirrored to the vcluster is always named the same as the original in the
	// host and is located in the DestinationNamespace
	return types.NamespacedName{
		Namespace: s.DestinationNamespace,
		Name:      pObj.GetName(),
	}
}

func (s *secretSyncer) translateUpdateUp(pObj, vObj *corev1.Secret) *corev1.Secret {
	var updated *corev1.Secret

	// sync annotations
	// We sync all of them from the host and remove any added in the vcluster
	if !equality.Semantic.DeepEqual(vObj.GetAnnotations(), pObj.GetAnnotations()) {
		updated = newIfNil(updated, vObj)
		updated.Annotations = pObj.GetAnnotations()
	}

	// sync lables
	// We sync all of them from the host, add one more to be able to detect
	// secrets synced by this plugin, and we remove any added in the vcluster
	expectedLabels := map[string]string{
		ManagedHostSecret: constants.PluginName,
	}
	for k, v := range pObj.GetLabels() {
		expectedLabels[k] = v
	}
	if !equality.Semantic.DeepEqual(vObj.GetLabels(), expectedLabels) {
		updated = newIfNil(updated, vObj)
		updated.Labels = expectedLabels
	}

	// sync data
	if !equality.Semantic.DeepEqual(vObj.Data, pObj.Data) {
		updated = newIfNil(updated, vObj)
		updated.Data = pObj.Data
	}

	// sync string data
	if !equality.Semantic.DeepEqual(vObj.StringData, pObj.StringData) {
		updated = newIfNil(updated, vObj)
		updated.StringData = pObj.StringData
	}

	return updated
}

func newIfNil(updated *corev1.Secret, pObj *corev1.Secret) *corev1.Secret {
	if updated == nil {
		return pObj.DeepCopy()
	}
	return updated
}
