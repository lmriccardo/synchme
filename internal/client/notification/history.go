package notification

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lmriccardo/synchme/internal/client/config"
	"github.com/lmriccardo/synchme/internal/client/consts"
	"github.com/lmriccardo/synchme/internal/utils"
)

type History struct {
	Path        string   // The path to the history folder
	ContentFile string   // The file containing the content
	Content     []string // A list of all synchronizable files and folders
}

// createSyncdFolders creates the top-level folders in the history folder
func createSyncdFolders(h *History) {
	// For each folder in the history content create the corresponding .dir
	for _, path := range h.Content {
		dir_folder_name := filepath.Base(path) + ".dir"
		syncd_folder := filepath.Join(h.Path, dir_folder_name)
		utils.MkdirNoErr(syncd_folder, 0777, false)
		utils.INFO("Created folder ", syncd_folder)
	}
}

// setupFolders setups the syncd folder with an initial history
func setupFolders(h *History, conf *config.ClientConf) {
	// Create the paths for the syncd folder and the syncd list file
	h.Path = filepath.Join(os.Getenv(consts.SYNCHME_FOLDER), consts.SYNCHME_HISTORY_FOLDER)
	h.ContentFile = filepath.Join(h.Path, consts.SYNCHME_HISTORY_CONTENT)

	// Create the syncd folder if it does not exists, otherwise continue
	utils.MkdirNoErr(h.Path, os.ModePerm, true)

	// Read the h_content file if exists. The h_content file contains whats
	// inside the history folder, in particular which files and folders.
	h_content := map[string]string{}
	if file, err := os.Open(h.ContentFile); err != nil {
		utils.WARN("Error when opening content file: ", err)
	} else {
		decoder := gob.NewDecoder(file)
		if err := decoder.Decode(&h_content); err != nil {
			utils.ERROR("Gob decoding failed: ", err)
		}
	}

	// Now, the setup shall check that every files and folders the user wants to synchronize
	// (as well as those belonging to the target remote folder which contains pulled updates
	// from other clients) already appears in the history. For every file that does not appear
	// entries in the history shall be created.
	target_sync_folder := os.Getenv(consts.SYNCHME_SYNC_FOLDER)
	h.Content = utils.ListFolder(target_sync_folder)
	h.Content = append(h.Content, conf.FS_Notification.Paths...)

	// Create the syncd folders before doing any other operation
	createSyncdFolders(h)

	// Get all the tree of contents in each folder
	paths := utils.MultiWalkDir(h.Content...)

	// We need to relativize all the path in the list of paths
	paths = utils.RelativizePaths(paths)

	// For each path resulting from the previous walkdir, compute the SHA-256 HASH
	// of the absolute path and check if it is present in the previous content map
	for _, sync_path := range paths {
		hashed_path := utils.ComputeSHA256(sync_path)
		if _, ok := h_content[hashed_path]; !ok {
			// Must create the history of the current sync path
			fmt.Printf("%s not found!\n", sync_path)
		}
	}
}

// LoadHistory initializes/load the local history
func LoadHistory(conf *config.ClientConf) *History {
	history := &History{}
	setupFolders(history, conf) // Setup all the folders for the history
	return history
}
