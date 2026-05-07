package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type documentationEntry struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Path        string `json:"path"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

var documentationCatalog = []documentationEntry{
	{ID: "documentation-index", Title: "Documentation Hub", Path: "doc/Documentation_Index.md", Category: "Start Here", Description: "Central agenda with role-based and question-based navigation."},
	{ID: "beginner-ui-workflow", Title: "Beginner UI Workflow", Path: "doc/Beginner_UI_Workflow.md", Category: "Beginner", Description: "Hands-on benchmark, baseline, and compare workflow with cloud runner explanation."},
	{ID: "readme", Title: "README", Path: "README.md", Category: "Start Here", Description: "Product overview, quick start, API and CLI entry points."},
	{ID: "architecture", Title: "Architecture", Path: "doc/Architecture.md", Category: "Core", Description: "Execution engine, distributed model, and data aggregation architecture."},
	{ID: "azure-deployment-design", Title: "Azure Deployment Design", Path: "doc/Azure_Deployment_Design.md", Category: "Azure", Description: "Control-plane model, auth, RBAC, and deployment flow on Azure."},
	{ID: "azure-managed-redis-design", Title: "Azure Managed Redis Design", Path: "doc/Azure_Managed_Redis_Design.md", Category: "Azure", Description: "AMR modes, networking patterns, and target integration details."},
	{ID: "azure-provider-design", Title: "Azure Provider Design", Path: "doc/Azure_Provider_Design.md", Category: "Azure", Description: "Provisioning state machine, synchronization, and failure handling."},
	{ID: "azure-deployment-methods", Title: "Azure Deployment Methods", Path: "doc/Azure_Deployment_Methods.md", Category: "Azure", Description: "Why SDK plus Bicep is used and AMR deployment caveats."},
	{ID: "project-status", Title: "Project Status", Path: "PROJECT_STATUS.md", Category: "Reference", Description: "Feature implementation status and phase completion overview."},
	{ID: "roadmap", Title: "Roadmap", Path: "ROADMAP.md", Category: "Reference", Description: "Planned improvements and future milestones."},
	{ID: "implementation-plan", Title: "Implementation Plan", Path: "IMPLEMENTATION_PLAN.md", Category: "Reference", Description: "Implementation phases and work breakdown."},
	{ID: "project-goals", Title: "Project Goals", Path: "Project_Goals.md", Category: "Reference", Description: "Long-term product goals and guiding principles."},
}

func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	root := s.resolveDocsRoot()
	entries := make([]map[string]interface{}, 0, len(documentationCatalog))

	for _, doc := range documentationCatalog {
		fullPath := filepath.Join(root, filepath.FromSlash(doc.Path))
		_, err := os.Stat(fullPath)
		entries = append(entries, map[string]interface{}{
			"id":          doc.ID,
			"title":       doc.Title,
			"path":        doc.Path,
			"category":    doc.Category,
			"description": doc.Description,
			"available":   err == nil,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i]["title"].(string) < entries[j]["title"].(string)
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"docs": entries,
	})
}

func (s *Server) handleDoc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/docs/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusBadRequest, "Invalid documentation ID")
		return
	}

	var selected *documentationEntry
	for _, doc := range documentationCatalog {
		if doc.ID == id {
			d := doc
			selected = &d
			break
		}
	}

	if selected == nil {
		writeError(w, http.StatusNotFound, "Documentation not found")
		return
	}

	root := s.resolveDocsRoot()
	fullPath := filepath.Join(root, filepath.FromSlash(selected.Path))
	content, err := os.ReadFile(fullPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "Documentation file not available")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":          selected.ID,
		"title":       selected.Title,
		"path":        selected.Path,
		"category":    selected.Category,
		"description": selected.Description,
		"content":     string(content),
	})
}

func (s *Server) resolveDocsRoot() string {
	if s.webDir != "" {
		return filepath.Clean(filepath.Join(s.webDir, "..", ".."))
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}
