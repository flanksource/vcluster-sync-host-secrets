package syncers

import (
	"testing"

	"github.com/loft-sh/vcluster-sdk/syncer"
	synccontext "github.com/loft-sh/vcluster-sdk/syncer/context"
	generictesting "github.com/loft-sh/vcluster-sdk/syncer/testing"
	"github.com/loft-sh/vcluster-sdk/syncer/translator"
	"github.com/loft-sh/vcluster-sdk/translate"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSyncDown(t *testing.T) {
	baseSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test",
		},
	}
	updatedSecret := &corev1.Secret{
		ObjectMeta: baseSecret.ObjectMeta,
		StringData: map[string]string{
			"test": "test",
		},
	}
	syncedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      translate.PhysicalName(baseSecret.Name, baseSecret.Namespace),
			Namespace: "test",
			Annotations: map[string]string{
				translator.NameAnnotation:      baseSecret.Name,
				translator.NamespaceAnnotation: baseSecret.Namespace,
			},
			Labels: map[string]string{
				translate.NamespaceLabel: baseSecret.Namespace,
			},
		},
	}
	updatedSyncedSecret := &corev1.Secret{
		ObjectMeta: syncedSecret.ObjectMeta,
		StringData: updatedSecret.StringData,
	}
	basePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: baseSecret.Namespace,
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "test",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: baseSecret.Name,
						},
					},
				},
			},
		},
	}

	generictesting.RunTests(t, []*generictesting.SyncTest{
		{
			Name: "Unused secret",
			InitialVirtualState: []runtime.Object{
				baseSecret,
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Secret"): {
					syncedSecret,
				},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, newSyncer)
				_, err := syncer.(*SecretSyncer).SyncDown(syncCtx, baseSecret)
				assert.NilError(t, err)
			},
		},
		{
			Name: "Used secret",
			InitialVirtualState: []runtime.Object{
				baseSecret,
				basePod,
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Secret"): {
					syncedSecret,
				},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, newSyncer)
				_, err := syncer.(*SecretSyncer).SyncDown(syncCtx, baseSecret)
				assert.NilError(t, err)
			},
		},
		{
			Name: "Update secret",
			InitialVirtualState: []runtime.Object{
				updatedSecret,
			},
			InitialPhysicalState: []runtime.Object{
				syncedSecret,
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				corev1.SchemeGroupVersion.WithKind("Secret"): {
					updatedSyncedSecret,
				},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, newSyncer)
				_, err := syncer.(*SecretSyncer).Sync(syncCtx, syncedSecret, updatedSecret)
				assert.NilError(t, err)
			},
		},
	})
}

func TestSyncUp(t *testing.T) {
	baseSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test",
		},
	}
	updatedSecret := &corev1.Secret{
		ObjectMeta: baseSecret.ObjectMeta,
		StringData: map[string]string{
			"test": "test",
		},
	}
	immutable := true
	syncedSecret := &corev1.Secret{
		ObjectMeta: baseSecret.ObjectMeta,
		Immutable:  &immutable,
	}
	syncedUpdatedSecret := &corev1.Secret{
		ObjectMeta: baseSecret.ObjectMeta,
		StringData: updatedSecret.StringData,
		Immutable:  &immutable,
	}

	generictesting.RunTests(t, []*generictesting.SyncTest{
		{
			Name: "Create secret",
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
				_, err := syncer.(*SecretSyncer).SyncUp(syncCtx, baseSecret)
				assert.NilError(t, err)
			},
		},
		{
			Name: "Update secret",
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
				_, err := syncer.(*SecretSyncer).Sync(syncCtx, updatedSecret, baseSecret)
				assert.NilError(t, err)
			},
		},
	})
}

func newSyncer(ctx *synccontext.RegisterContext) (syncer.Base, error) {
	return NewSecretSyncer(ctx), nil
}
