package syncers

import (
	"context"

	"github.com/loft-sh/vcluster-sdk/syncer"
	syncercontext "github.com/loft-sh/vcluster-sdk/syncer/context"
	"github.com/loft-sh/vcluster-sdk/syncer/translator"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewSecretSyncer(ctx *syncercontext.RegisterContext) *SecretSyncer {
	return &SecretSyncer{
		NamespacedTranslator: translator.NewNamespacedTranslator(ctx, "secret", &corev1.Secret{}),
	}
}

type SecretSyncer struct {
	translator.NamespacedTranslator
	syncer.UpSyncer
}

// Make sure the interface is implemented
var _ syncer.Syncer = &SecretSyncer{}

func (s *SecretSyncer) SyncDown(ctx *syncercontext.SyncContext, vObj client.Object) (ctrl.Result, error) {
	return s.SyncDownCreate(ctx, vObj, s.translate(vObj.(*corev1.Secret)))
}

func (s *SecretSyncer) SyncUp(ctx *syncercontext.SyncContext, pObj client.Object) (ctrl.Result, error) {
	managed, err := s.IsManaged(pObj)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	// Only sync secrets originating in the host cluster to the vcluster
	if !managed {
		pSecret := pObj.(*corev1.Secret)
		pSecret.ObjectMeta.ResourceVersion = ""
		immutable := true
		pSecret.Immutable = &immutable
		err = ctx.VirtualClient.Create(context.Background(), pSecret)
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{Requeue: true}, nil
}

func (s *SecretSyncer) SyncUpUpdate(ctx *syncercontext.SyncContext, pObj client.Object, vObj client.Object) (ctrl.Result, error) {
	managed, err := s.IsManaged(pObj)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	// Only sync secrets originating in the host cluster to the vcluster
	if !managed {
		pSecret := pObj.(*corev1.Secret)
		immutable := true
		pSecret.Immutable = &immutable
		err = ctx.PhysicalClient.Update(context.Background(), pObj)
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

func (s *SecretSyncer) Sync(ctx *syncercontext.SyncContext, pObj client.Object, vObj client.Object) (ctrl.Result, error) {
	pSecret := pObj.(*corev1.Secret)
	vSecret := vObj.(*corev1.Secret)
	if pSecret.Immutable != nil && vSecret.Immutable != nil && *pSecret.Immutable && !*vSecret.Immutable {
		// if the Secret in the host is Immutable, while Secret in vcluster
		// is not Immutable, then we need to delete it from the host to reconcile
		// it into the expected state. We force requeue to trigger recreation.
		_, err := syncer.DeleteObject(ctx, pObj)
		return ctrl.Result{Requeue: true}, err
	}
	managed, err := s.IsManaged(pSecret)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if managed {
		return s.SyncDownUpdate(ctx, vObj, s.translateUpdate(pSecret, vSecret))
	}
	return s.SyncUpUpdate(ctx, s.translateUpdate(vSecret, pSecret), pObj)
}

func (s *SecretSyncer) translate(vObj client.Object) *corev1.Secret {
	return s.TranslateMetadata(vObj).(*corev1.Secret)
}

// translateUpdate returns nil if the host side Secret doesn't need to be updated,
// otherwise it returns an updated Secret.
// Note: the caller has to cover the case where the vObj.Immutable is true, and pObj.Immutable is false
func (s *SecretSyncer) translateUpdate(pObj, vObj *corev1.Secret) *corev1.Secret {
	var updated *corev1.Secret

	// check if the annotations or labels have changed
	changed, updatedAnnotations, updatedLabels := s.TranslateMetadataUpdate(vObj, pObj)
	if changed {
		updated = newIfNil(updated, pObj)
		updated.Labels = updatedLabels
		updated.Annotations = updatedAnnotations
	}

	// check if the data has changed
	if !equality.Semantic.DeepEqual(vObj.Data, pObj.Data) {
		updated = newIfNil(updated, pObj)
		updated.Data = vObj.Data
	}

	// check if the string data has changed
	if !equality.Semantic.DeepEqual(vObj.StringData, pObj.StringData) {
		updated = newIfNil(updated, pObj)
		updated.StringData = vObj.StringData
	}

	// check if the Immutable field has changed
	// Note: the caller has to cover the case where the vObj.Immutable is true, and pObj.Immutable is false
	if !equality.Semantic.DeepEqual(vObj.Immutable, pObj.Immutable) {
		updated = newIfNil(updated, pObj)
		updated.Immutable = vObj.Immutable
	}

	return updated
}

func newIfNil(updated *corev1.Secret, pObj *corev1.Secret) *corev1.Secret {
	if updated == nil {
		return pObj.DeepCopy()
	}
	return updated
}
