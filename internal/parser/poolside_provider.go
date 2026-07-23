// ABOUTME: Provider for Poolside Agent CLI (`pool`) trajectory NDJSON files.
// ABOUTME: Discovers trajectories under <root>/trajectories/, watches for
// ABOUTME: changes, and fingerprints composite trajectory + session metadata.
package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// newPoolsideProviderFactory creates a provider factory for Poolside Agent CLI.
// Poolside Agent CLI stores sessions as trajectory NDJSON files under
// <root>/trajectories/ with pattern trajectory-<type>_<uuid>.ndjson.
func newPoolsideProviderFactory(def AgentDef) ProviderFactory {
	return NewSingleFileProviderFactory(
		def,
		poolsideProviderCapabilities(),
		func(cfg ProviderConfig) singleFileSourceSet {
			return NewSingleFileSourceSet(
				def.Type,
				cfg.Roots,
				WithFileDiscovery(poolsideDiscoverFiles),
				WithFileWatchRoots(poolsideWatchRoots),
				WithFileChangedPathClassifier(
					func(root, path string, allowMissing bool) (singleFileMatch, bool) {
						return poolsideClassifyPath(root, path, allowMissing)
					},
				),
				WithFileLookup(func(root, rawID string) (singleFileMatch, bool) {
					return poolsideFindFile(root, rawID)
				}),
				WithFileFingerprint(func(src singleFileSource) (SourceFingerprint, error) {
					return poolsideFingerprintSource(src.Path)
				}),
				WithFileParse(func(src singleFileSource, req ParseRequest) ([]ParseResult, []string, error) {
					return poolsideParseFile(src, req)
				}),
			)
		},
	)
}

// poolsideDiscoverFiles finds all trajectory NDJSON files under a root.
// root is the poolside data directory; trajectories live under
// <root>/trajectories/.
func poolsideDiscoverFiles(root string) []singleFileMatch {
	trajectoriesDir := filepath.Join(root, "trajectories")
	entries, err := os.ReadDir(trajectoriesDir)
	if err != nil {
		return nil
	}

	var matches []singleFileMatch
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "trajectory-") || !strings.HasSuffix(name, ".ndjson") {
			continue
		}
		path := filepath.Join(trajectoriesDir, name)
		matches = append(matches, singleFileMatch{
			Path: path,
		})
	}
	return matches
}

// poolsideWatchRoots creates watch plans for each root.
// Watches the trajectories/ subdirectory for .ndjson changes.
func poolsideWatchRoots(roots []string) []WatchRoot {
	out := make([]WatchRoot, 0, len(roots))
	for _, root := range roots {
		trajectoriesDir := filepath.Join(root, "trajectories")
		out = append(out, WatchRoot{
			Path:         trajectoriesDir,
			Recursive:    false,
			IncludeGlobs: []string{"*.ndjson"},
			DebounceKey:  "poolside:trajectories:" + root,
		})
	}
	return out
}

// poolsideClassifyPath maps a changed trajectory file path back to its source.
func poolsideClassifyPath(
	root, path string, allowMissing bool,
) (singleFileMatch, bool) {
	root = filepath.Clean(root)
	path = filepath.Clean(path)

	trajectoriesDir := filepath.Join(root, "trajectories")
	rel, err := filepath.Rel(trajectoriesDir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return singleFileMatch{}, false
	}

	// Must be a direct child of trajectories/.
	if strings.Contains(rel, string(filepath.Separator)) {
		return singleFileMatch{}, false
	}

	name := filepath.Base(rel)
	if !strings.HasPrefix(name, "trajectory-") || !strings.HasSuffix(name, ".ndjson") {
		return singleFileMatch{}, false
	}

	if allowMissing {
		return singleFileMatch{Path: path}, true
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return singleFileMatch{}, false
	}
	return singleFileMatch{Path: path}, true
}

// poolsideFindFile finds a trajectory file by raw ID under the root.
// The raw ID is the part after "poolside:" prefix (e.g. "standalone_<uuid>").
func poolsideFindFile(root, rawID string) (singleFileMatch, bool) {
	trajectoriesDir := filepath.Join(root, "trajectories")
	path := filepath.Join(trajectoriesDir, "trajectory-"+rawID+".ndjson")
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return singleFileMatch{Path: path}, true
	}
	return singleFileMatch{}, false
}

// poolsideFingerprintSource computes a fingerprint for a trajectory file.
func poolsideFingerprintSource(sourcePath string) (SourceFingerprint, error) {
	hash, size, mtime, err := hashPoolsideSourceFile(sourcePath)
	if err != nil {
		return SourceFingerprint{}, fmt.Errorf("hashing trajectory file: %w", err)
	}
	return SourceFingerprint{
		Size:    size,
		MTimeNS: mtime,
		Hash:    hash,
	}, nil
}

// poolsideParseFile parses a single Poolside trajectory file.
func poolsideParseFile(
	src singleFileSource, req ParseRequest,
) ([]ParseResult, []string, error) {
	sess, msgs, usageEvents, err := parsePoolsideSession(
		src.Path, req.Source.ProjectHint, req.Machine,
	)
	if err != nil {
		return nil, nil, err
	}
	if sess == nil {
		return nil, nil, nil
	}

	// Apply fingerprint metadata.
	if req.Fingerprint.Size > 0 {
		sess.File.Size = req.Fingerprint.Size
	}
	if req.Fingerprint.MTimeNS > 0 {
		sess.File.Mtime = req.Fingerprint.MTimeNS
	}
	if req.Fingerprint.Hash != "" {
		sess.File.Hash = req.Fingerprint.Hash
	}

	return []ParseResult{{
		Session:     *sess,
		Messages:    msgs,
		UsageEvents: usageEvents,
	}}, nil, nil
}

// poolsideProviderCapabilities declares what the poolside provider supports.
func poolsideProviderCapabilities() Capabilities {
	return Capabilities{
		Source: SourceCapabilities{
			DiscoverSources:      CapabilitySupported,
			WatchSources:         CapabilitySupported,
			ClassifyChangedPath:  CapabilitySupported,
			FindSource:           CapabilitySupported,
			CompositeFingerprint: CapabilitySupported,
			MultiSessionSource:   CapabilityNotApplicable,
			PerSessionErrors:     CapabilityNotApplicable,
			ExcludedSessions:     CapabilityNotApplicable,
			ForceReplaceOnParse:  CapabilityNotApplicable,
		},
		Content: ContentCapabilities{
			FirstMessage:         CapabilitySupported,
			SessionName:          CapabilitySupported,
			Cwd:                  CapabilitySupported,
			Model:                CapabilitySupported,
			Thinking:             CapabilitySupported,
			ToolCalls:            CapabilitySupported,
			ToolResults:          CapabilitySupported,
			AggregateUsageEvents: CapabilitySupported,
			TerminationStatus:    CapabilitySupported,
		},
	}
}
