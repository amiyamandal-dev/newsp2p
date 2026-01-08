package integration

import (
	"os"
	"testing"
	"time"

	"github.com/amiyamandal-dev/newsp2p/internal/auth"
	"github.com/amiyamandal-dev/newsp2p/internal/repository/badger"
	"github.com/amiyamandal-dev/newsp2p/internal/service"
	"github.com/amiyamandal-dev/newsp2p/pkg/logger"
	"github.com/amiyamandal-dev/newsp2p/tests/mocks"
)

type TestEnv struct {
	DB             *badger.DB
	UserRepo       *badger.UserRepo
	ArticleRepo    *badger.ArticleRepo
	UserService    *service.UserService
	ArticleService *service.ArticleService
	JWTManager     *auth.JWTManager
	Cleanup        func()
}

func SetupTestEnv(t *testing.T) *TestEnv {
	// Create temp dir for DB
	tmpDir, err := os.MkdirTemp("", "newsp2p-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Init DB
	db, err := badger.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to init badger db: %v", err)
	}

	// Init Logger
	log, _ := logger.New("error", "text")

	// Init Repos
	userRepo := badger.NewUserRepo(db)
	articleRepo := badger.NewArticleRepo(db)

	// Init Auth
	jwtManager := auth.NewJWTManager("test-secret", time.Hour, 24*time.Hour)
	signer := auth.NewArticleSigner()

	// Init Mocks
	mockIPFS := mocks.NewMockIPFSClient()

	// Init Services
	userService := service.NewUserService(userRepo, jwtManager, 10, log)
	articleService := service.NewArticleService(
		articleRepo,
		userRepo,
		mockIPFS, // Mock IPFS
		nil,      // No broadcaster for integration tests yet
		signer,
		nil, // Search Mock
		log,
	)

	return &TestEnv{
		DB:             db,
		UserRepo:       userRepo,
		ArticleRepo:    articleRepo,
		UserService:    userService,
		ArticleService: articleService,
		JWTManager:     jwtManager,
		Cleanup: func() {
			db.Close()
			os.RemoveAll(tmpDir)
		},
	}
}
