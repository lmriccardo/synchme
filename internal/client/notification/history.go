package notification

import "github.com/lmriccardo/synchme/internal/client/config"

type History struct {
}

// InitHistory initializes/load the local history
func InitHistory(conf *config.ClientConf) *History {
	// // Take the path of the synchronization folder and checks (1) if it
	// // already exists, otherwise
	// sync_folder := conf.FS_Notification.SyncFolder

	return &History{}
}
