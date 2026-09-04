package tus

import (
	"context"

	"github.com/tus/tusd/v2/pkg/handler"
)

// DeferredConcater replaces the filestore's synchronous concatenation. tusd's
// default implementation copies every partial upload into the final file inside
// the final POST request. For a multi-hundred-gigabyte upload that means tens of
// minutes of a silent HTTP request and twice the file size in temporary disk;
// when the connection drops the client retries the POST, races the finalizer's
// cleanup of the partials, and ends with a 404.
//
// This concater does nothing: tusd has already verified in sizeOfUploads that
// every partial exists and is complete, and it emits CompleteUploads with the
// partial IDs. The EventProcessor then assembles the file asynchronously
// (assembleConcat), appending partials one at a time and deleting each after it
// has been appended.
//
// Side effect to be aware of: the final upload's .info sidecar keeps Offset 0
// until cleanup. Nothing reads it — recoveryFileInfo overrides Offset with the
// database size and the browser never HEADs the final upload URL.
type DeferredConcater struct{}

var _ handler.ConcaterDataStore = DeferredConcater{}

// AsConcatableUpload implements handler.ConcaterDataStore.
func (DeferredConcater) AsConcatableUpload(upload handler.Upload) handler.ConcatableUpload {
	return deferredConcat{}
}

type deferredConcat struct{}

// ConcatUploads implements handler.ConcatableUpload without copying any bytes.
func (deferredConcat) ConcatUploads(ctx context.Context, partials []handler.Upload) error {
	return nil
}
