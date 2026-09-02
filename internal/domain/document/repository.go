package document

import "context"

// DocumentRepository — интерфейс хранилища документов (ADR-0006).
type DocumentRepository interface {
	SavePackage(ctx context.Context, pkg *DocumentPackage) error
	GetPackage(ctx context.Context, id PackageID) (*DocumentPackage, error)
	ListPackagesByConfig(ctx context.Context, configID string) ([]*DocumentPackage, error)
	SaveDocument(ctx context.Context, doc *Document) error
	GetDocument(ctx context.Context, id DocumentID) (*Document, error)
	DeletePackage(ctx context.Context, id PackageID) error
}
