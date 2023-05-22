package syncers

import (
	"testing"

	"github.com/flanksource/vcluster-sync-host-secrets/constants"
	"github.com/loft-sh/vcluster-sdk/syncer"
	synccontext "github.com/loft-sh/vcluster-sdk/syncer/context"
	generictesting "github.com/loft-sh/vcluster-sdk/syncer/testing"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSyncUp(t *testing.T) {
	baseSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test",
			Annotations: map[string]string{
				constants.SyncAnnotation: "true",
			},
		},
	}
	syncedSecret := &corev1.Secret{
		ObjectMeta: baseSecret.ObjectMeta,
	}
	syncedSecret.Labels = map[string]string{
		ManagedHostSecret: constants.PluginName,
	}
	newNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "other-test",
		},
	}
	namespaceSecret := &corev1.Secret{
		ObjectMeta: baseSecret.ObjectMeta,
	}
	namespaceSecret.ObjectMeta.Annotations = map[string]string{
		constants.SyncAnnotation:      "true",
		constants.NamespaceAnnotation: newNamespace.Name,
	}
	syncedNamespaceSecret := &corev1.Secret{
		ObjectMeta: namespaceSecret.ObjectMeta,
	}
	syncedNamespaceSecret.Namespace = newNamespace.Name
	syncedNamespaceSecret.Labels = map[string]string{
		ManagedHostSecret: constants.PluginName,
	}

	updatedSecret := &corev1.Secret{
		ObjectMeta: baseSecret.ObjectMeta,
		StringData: map[string]string{
			"test": "test",
		},
	}
	syncedUpdatedSecret := &corev1.Secret{
		ObjectMeta: syncedSecret.ObjectMeta,
		StringData: updatedSecret.StringData,
	}

	generictesting.RunTests(t, []*generictesting.SyncTest{
		{
			Name: "Create virtual secret",
			InitialPhysicalState: []runtime.Object{
				baseSecret,
			},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Secret"): {
					syncedSecret,
				},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, newSyncer)
				_, err := syncer.(*secretSyncer).SyncUp(syncCtx, baseSecret)
				assert.NilError(t, err)
			},
		},
		{
			Name: "Create virtual secret in different namespace",
			InitialPhysicalState: []runtime.Object{
				namespaceSecret,
			},
			InitialVirtualState: []runtime.Object{
				newNamespace,
			},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Secret"): {
					syncedNamespaceSecret,
				},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, newSyncer)
				_, err := syncer.(*secretSyncer).SyncUp(syncCtx, namespaceSecret)
				assert.NilError(t, err)
			},
		},
		{
			Name: "Update virtual secret",
			InitialPhysicalState: []runtime.Object{
				updatedSecret,
			},
			InitialVirtualState: []runtime.Object{
				baseSecret,
			},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Secret"): {
					syncedUpdatedSecret,
				},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, newSyncer)
				_, err := syncer.(*secretSyncer).Sync(syncCtx, updatedSecret, baseSecret)
				assert.NilError(t, err)
			},
		},
	})
}

func newSyncer(ctx *synccontext.RegisterContext) (syncer.Base, error) {
	return NewSecretSyncer(ctx, ctx.TargetNamespace), nil
}
