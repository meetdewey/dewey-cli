package api

// Collection mirrors the Dewey collections schema. Fields use json tags that
// match the API verbatim — the CLI never reshapes API responses.
type Collection struct {
	ID                     string  `json:"id"`
	ProjectID              string  `json:"projectId"`
	Name                   string  `json:"name"`
	Visibility             string  `json:"visibility"`
	ChunkSize              int     `json:"chunkSize"`
	ChunkOverlap           int     `json:"chunkOverlap"`
	EmbeddingModel         string  `json:"embeddingModel"`
	Description            *string `json:"description"`
	DescriptionDocCount    *int    `json:"descriptionDocCount"`
	EnableSummarization    bool    `json:"enableSummarization"`
	EnableCaptioning       bool    `json:"enableCaptioning"`
	LLMModel               *string `json:"llmModel"`
	LastSummarizationModel *string `json:"lastSummarizationModel"`
	LastCaptioningModel    *string `json:"lastCaptioningModel"`
	Instructions           *string `json:"instructions"`
	EnableDeduplication    bool    `json:"enableDeduplication"`
	LastDeduplicationAt    *string `json:"lastDeduplicationAt"`
	DuplicateGroupCount    int     `json:"duplicateGroupCount"`
	EnableReranking        bool    `json:"enableReranking"`
	CreatedAt              string  `json:"createdAt"`
	DeletedAt              *string `json:"deletedAt"`
}

type CollectionStats struct {
	DocCount             int            `json:"docCount"`
	TotalFileSizeBytes   int64          `json:"totalFileSizeBytes"`
	TotalSections        int            `json:"totalSections"`
	TotalChunks          int            `json:"totalChunks"`
	StatusCounts         map[string]int `json:"statusCounts"`
	SummarizedCount      int            `json:"summarizedCount"`
	CaptionedCount       int            `json:"captionedCount"`
	ClaimsExtractedCount int            `json:"claimsExtractedCount"`
	TotalClaimsCount     int            `json:"totalClaimsCount"`
}

type CreateCollectionInput struct {
	Name           string `json:"name"`
	ProjectID      string `json:"projectId"`
	Visibility     string `json:"visibility,omitempty"`
	ChunkSize      int    `json:"chunkSize,omitempty"`
	ChunkOverlap   int    `json:"chunkOverlap,omitempty"`
	EmbeddingModel string `json:"embeddingModel,omitempty"`
}

type UpdateCollectionInput struct {
	Name                *string `json:"name,omitempty"`
	Visibility          *string `json:"visibility,omitempty"`
	ChunkSize           *int    `json:"chunkSize,omitempty"`
	ChunkOverlap        *int    `json:"chunkOverlap,omitempty"`
	EmbeddingModel      *string `json:"embeddingModel,omitempty"`
	Description         *string `json:"description,omitempty"`
	EnableSummarization *bool   `json:"enableSummarization,omitempty"`
	EnableCaptioning    *bool   `json:"enableCaptioning,omitempty"`
	LLMModel            *string `json:"llmModel,omitempty"`
	Instructions        *string `json:"instructions,omitempty"`
	EnableDeduplication *bool   `json:"enableDeduplication,omitempty"`
	EnableReranking     *bool   `json:"enableReranking,omitempty"`
}

type Document struct {
	ID                    string                 `json:"id"`
	CollectionID          string                 `json:"collectionId"`
	Filename              string                 `json:"filename"`
	StorageKey            string                 `json:"storageKey"`
	MarkdownStorageKey    *string                `json:"markdownStorageKey"`
	Status                string                 `json:"status"`
	FileSizeBytes         *int64                 `json:"fileSizeBytes"`
	MarkdownFileSizeBytes *int64                 `json:"markdownFileSizeBytes"`
	SectionCount          *int                   `json:"sectionCount"`
	ChunkCount            *int                   `json:"chunkCount"`
	ContentHash           *string                `json:"contentHash"`
	ErrorMessage          *string                `json:"errorMessage"`
	DuplicateGroupID      *string                `json:"duplicateGroupId"`
	DuplicateRelationship *string                `json:"duplicateRelationship"`
	CoverageToCanonical   *float64               `json:"coverageToCanonical"`
	CoverageFromCanonical *float64               `json:"coverageFromCanonical"`
	Tags                  []string               `json:"tags"`
	Metadata              map[string]interface{} `json:"metadata"`
	CreatedAt             string                 `json:"createdAt"`
}

type Section struct {
	ID                   string  `json:"id"`
	DocumentID           string  `json:"documentId"`
	Title                string  `json:"title"`
	Level                int     `json:"level"`
	Summary              *string `json:"summary"`
	SummaryType          *string `json:"summaryType"`
	Position             int     `json:"position"`
	ChunkCount           int     `json:"chunkCount"`
	MarkdownOffsetStart  int     `json:"markdownOffsetStart"`
	MarkdownOffsetEnd    int     `json:"markdownOffsetEnd"`
	Content              *string `json:"content,omitempty"`
}

type Chunk struct {
	ID           string `json:"id"`
	SectionID    string `json:"sectionId"`
	DocumentID   string `json:"documentId"`
	CollectionID string `json:"collectionId"`
	Content      string `json:"content"`
	Position     int    `json:"position"`
	TokenCount   int    `json:"tokenCount"`
}

type RetrievalResult struct {
	Score float64 `json:"score"`
	Chunk struct {
		ID         string `json:"id"`
		Content    string `json:"content"`
		Position   int    `json:"position"`
		TokenCount int    `json:"tokenCount"`
	} `json:"chunk"`
	Section struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Level int    `json:"level"`
	} `json:"section"`
	Document struct {
		ID       string `json:"id"`
		Filename string `json:"filename"`
	} `json:"document"`
}

type SectionScanResult struct {
	Score   float64 `json:"score"`
	Section struct {
		ID             string  `json:"id"`
		Title          string  `json:"title"`
		Level          int     `json:"level"`
		Summary        *string `json:"summary"`
		SummaryType    *string `json:"summaryType"`
		Position       int     `json:"position"`
		ChunkCount     int     `json:"chunkCount"`
		MarkdownOffset struct {
			Start int `json:"start"`
			End   int `json:"end"`
		} `json:"markdownOffset"`
	} `json:"section"`
	Document struct {
		ID       string `json:"id"`
		Filename string `json:"filename"`
	} `json:"document"`
}

type SectionScanResponse struct {
	Results []SectionScanResult `json:"results"`
}

type ResearchSource struct {
	ChunkID      string `json:"chunkId"`
	Content      string `json:"content"`
	SectionID    string `json:"sectionId"`
	SectionTitle string `json:"sectionTitle"`
	SectionLevel int    `json:"sectionLevel"`
	DocumentID   string `json:"documentId"`
	Filename     string `json:"filename"`
}

type ResearchResult struct {
	Answer    string           `json:"answer"`
	SessionID string           `json:"sessionId"`
	Sources   []ResearchSource `json:"sources"`
}

type ResearchOptions struct {
	Depth    string                 `json:"depth,omitempty"`
	Model    string                 `json:"model,omitempty"`
	Tags     []string               `json:"tags,omitempty"`
	AnyTags  []string               `json:"anyTags,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type Claim struct {
	ID             string `json:"id"`
	SectionTitle   string `json:"sectionTitle"`
	SectionLineage string `json:"sectionLineage"`
	Text           string `json:"text"`
	Importance     int    `json:"importance"`
	Position       int    `json:"position"`
}

type DocumentClaims struct {
	DocumentID string  `json:"documentId"`
	Claims     []Claim `json:"claims"`
}

type ProviderKey struct {
	ID         string `json:"id"`
	ProjectID  string `json:"projectId"`
	Provider   string `json:"provider"`
	Name       string `json:"name"`
	KeyPreview string `json:"keyPreview"`
	CreatedAt  string `json:"createdAt"`
}

type CreateProviderKeyInput struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
	Name     string `json:"name"`
}

type DuplicateGroupMember struct {
	ID                    string   `json:"id"`
	Filename              string   `json:"filename"`
	Relationship          *string  `json:"relationship"`
	CoverageToCanonical   *float64 `json:"coverageToCanonical"`
	CoverageFromCanonical *float64 `json:"coverageFromCanonical"`
	CreatedAt             string   `json:"createdAt"`
}

type DuplicateGroup struct {
	ID                  string                 `json:"id"`
	CanonicalDocumentID string                 `json:"canonicalDocumentId"`
	DetectedAt          string                 `json:"detectedAt"`
	Members             []DuplicateGroupMember `json:"members"`
}

type DuplicateGroupList struct {
	Total int              `json:"total"`
	Items []DuplicateGroup `json:"items"`
}

type DuplicateRun struct {
	ID                      string  `json:"id"`
	Status                  string  `json:"status"`
	JobsEnqueued            *int    `json:"jobsEnqueued"`
	JobsProcessed           *int    `json:"jobsProcessed"`
	DuplicatesDetected      *int    `json:"duplicatesDetected"`
	DuplicateGroupsCreated  *int    `json:"duplicateGroupsCreated"`
	StartedAt               *string `json:"startedAt"`
	CompletedAt             *string `json:"completedAt"`
	Error                   *string `json:"error"`
	CreatedAt               string  `json:"createdAt"`
}

type DuplicateDetectResult struct {
	RunID        string `json:"runId"`
	Status       string `json:"status"`
	JobsEnqueued int    `json:"jobsEnqueued"`
	EnqueuedAt   string `json:"enqueuedAt"`
}

type ContradictionClaim struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Document struct {
		ID       string `json:"id"`
		Filename string `json:"filename"`
	} `json:"document"`
	SectionTitle string `json:"sectionTitle"`
}

type Contradiction struct {
	ID                   string               `json:"id"`
	Severity             string               `json:"severity"`
	Status               string               `json:"status"`
	Explanation          string               `json:"explanation"`
	SuggestedInstruction *string              `json:"suggestedInstruction"`
	ClusterTopicSummary  *string              `json:"clusterTopicSummary"`
	CreatedAt            string               `json:"createdAt"`
	Claims               []ContradictionClaim `json:"claims"`
}

type ContradictionList struct {
	Total int             `json:"total"`
	Items []Contradiction `json:"items"`
}

type ContradictionDetectResult struct {
	RunID      string `json:"runId"`
	Status     string `json:"status"`
	EnqueuedAt string `json:"enqueuedAt"`
}

type ContradictionRun struct {
	ID                  string  `json:"id"`
	Status              string  `json:"status"`
	ClaimsProcessed     *int    `json:"claimsProcessed"`
	ClustersAnalyzed    *int    `json:"clustersAnalyzed"`
	ContradictionsFound *int    `json:"contradictionsFound"`
	Model               *string `json:"model"`
	StartedAt           *string `json:"startedAt"`
	CompletedAt         *string `json:"completedAt"`
	Error               *string `json:"error"`
	CreatedAt           string  `json:"createdAt"`
}

type DocumentEvent struct {
	DocumentID string `json:"documentId"`
	Status     string `json:"status"`
	Filename   string `json:"filename,omitempty"`
	Error      string `json:"error,omitempty"`
}
