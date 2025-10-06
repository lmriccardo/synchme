package notification

import (
	"github.com/lmriccardo/synchme/internal/proto/filesync"
	"github.com/lmriccardo/synchme/internal/utils"
	"github.com/sergi/go-diff/diffmatchpatch"
)

func createUpdateMessage(event *NotificationEvent, meta *filesync.MessageMeta) []*filesync.SyncMessage {
	dw := diffmatchpatch.New()
	patches_str := dw.PatchToText(event.Patches)
	chunks := []*filesync.SyncMessage{} // Initialize the chunks

	// Set the correct update type from the event operation flag
	var update_type filesync.FileUpdate_UpdateType
	if event.Op.Has(Write) {
		update_type = filesync.FileUpdate_WRITE
	} else {
		update_type = filesync.FileUpdate_CREATE
	}

	// Create the slice of chunks from the text containing patches
	data_chunks := utils.ChunkData([]byte(patches_str), CHUNK_SIZE)
	for chunk_idx, chunk_data := range data_chunks {
		chunks = append(chunks, &filesync.SyncMessage{
			Meta: meta,
			Msg: &filesync.SyncMessage_Update{
				Update: &filesync.FileUpdate{
					Path:        event.Path,
					Data:        chunk_data,
					IsChunk:     len(data_chunks) > 1,
					ChunkIndex:  int32(chunk_idx),
					TotalChunks: int32(len(data_chunks)),
					IsFolder:    false,
					Type:        update_type,
				},
			},
		})

		// Increase the message counter internally (then it will be
		// decrease when the function exit)
		MESSAGE_COUNTER++
		meta.Identifier = MESSAGE_COUNTER
	}

	return chunks
}

func createRenameMessage(event *NotificationEvent, meta *filesync.MessageMeta) *filesync.SyncMessage {
	return &filesync.SyncMessage{Meta: meta, Msg: &filesync.SyncMessage_Rename{
		Rename: &filesync.FileRename{Path: event.Path, OldPath: event.OldPath},
	}}
}

func createRemoveMessage(event *NotificationEvent, meta *filesync.MessageMeta) *filesync.SyncMessage {
	return &filesync.SyncMessage{Meta: meta, Msg: &filesync.SyncMessage_Remove{
		Remove: &filesync.FileRemove{Path: event.Path},
	}}
}

func createSyncAck(meta *filesync.MessageMeta, path string) *filesync.SyncMessage {
	return &filesync.SyncMessage{Meta: meta, Msg: &filesync.SyncMessage_Ack{
		Ack: &filesync.SyncAck{
			Path:     path,
			ToClient: meta.OriginClient,
		},
	}}
}
