package ports

import (
	"context"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// Renderer defines the interface for rendering presentations and slides
type Renderer interface {
	RenderPresentation(ctx context.Context, presentation *entities.Presentation) ([]byte, error)
	RenderSlide(ctx context.Context, slide *entities.Slide) ([]byte, error)
}
