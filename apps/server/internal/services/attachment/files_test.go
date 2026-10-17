package attachment_service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"ricrac/config"
	"ricrac/ent"
	"ricrac/internal/auth"
	"ricrac/internal/common/errdefs"
	"ricrac/internal/services"
	mocks_services_events "ricrac/mocks/services/events"
	mocks_services_resolve "ricrac/mocks/services/resolve"
)

type FilesSuite struct {
	services.ServiceTestSuite
	resolve *mocks_services_resolve.ResolveServiceMock
	service *AttachmentService
	actor   *auth.Actor
}

func (suite *FilesSuite) SetupTest() {
	suite.ServiceTestSuite.SetupTest()
	suite.resolve = mocks_services_resolve.NewResolveServiceMock(suite.T())
	cfg := config.Config{PublicURL: "https://ricrac.test", MaxUploadSize: 1024}
	suite.service = NewAttachmentService(cfg, suite.MockRepo, suite.resolve, mocks_services_events.NewEventsServiceMock(suite.T()))
	suite.actor = auth.NewActor(uuid.New(), uuid.New(), auth.Member, auth.EveryAccess, nil, nil)
}

func TestFiles(t *testing.T) {
	suite.Run(t, new(FilesSuite))
}

func (suite *FilesSuite) TestUploadsAreLimited() {
	_, err := suite.service.PrepareFileUpload(suite.Ctx, suite.actor, &PrepareUploadInput{Name: "a.png", ContentType: "image/png", Size: 2048})
	suite.Equal("size is the file's size in bytes, at most 1024", errdefs.As(err).Message)
	_, err = suite.service.UploadFile(suite.Ctx, suite.actor, &UploadInput{Name: "a.txt", ContentType: "text/plain", Base64: ""})
	suite.Equal("The file must have 1 to 1024 bytes; upload bigger ones with prepare_file_upload", errdefs.As(err).Message)
}

func (suite *FilesSuite) TestOnlyUploadedFilesAreAttached() {
	issue, file := uuid.New(), uuid.New()
	suite.resolve.EXPECT().Issue(mock.Anything, suite.actor, "ENG-1").Return(issue, nil)
	suite.MockAttachmentRepo.EXPECT().StoredFile(mock.Anything, suite.actor.Workspace, file).Return(&ent.File{ID: file, Name: "a.png"}, false, nil)
	_, err := suite.service.CreateFromUpload(suite.Ctx, suite.actor, &FromUploadInput{Issue: "ENG-1", FileID: file.String()})
	suite.Equal("The file hasn't been uploaded yet; PUT it to its uploadUrl first", errdefs.As(err).Message)
}

func TestFileNames(t *testing.T) {
	for value, want := range map[string]string{"shot.png": "shot.png", "/tmp/dir/shot.png": "shot.png", `C:\a\b.txt`: "b.txt", " a\x00b ": "ab"} {
		if got, err := checkFileName(value); err != nil || got != want {
			t.Errorf("%q: got %q, %v", value, got, err)
		}
	}
	if _, err := checkFileName("dir/"); err == nil {
		t.Error("a directory should fail")
	}
}

func TestContentTypes(t *testing.T) {
	if got, err := checkContentType(" Image/PNG "); err != nil || got != "image/png" {
		t.Errorf("got %q, %v", got, err)
	}
	for _, value := range []string{"png", "image/", "text/plain; charset=utf-8"} {
		if _, err := checkContentType(value); err == nil {
			t.Errorf("%q should fail", value)
		}
	}
}

func TestBase64Variants(t *testing.T) {
	for _, value := range []string{"aGk=", "aGk", "data:text/plain;base64,aGk=", "aG\nk="} {
		if got, err := decodeBase64(value); err != nil || string(got) != "hi" {
			t.Errorf("%s: got %q, %v", value, got, err)
		}
	}
	if _, err := decodeBase64("not base64!"); err == nil {
		t.Error("should fail")
	}
}

func TestFilesInMarkdown(t *testing.T) {
	if got := fileMarkdown("[shot].png", "image/png", "u"); got != "![shot.png](u)" {
		t.Errorf("got %s", got)
	}
	if got := fileMarkdown("log.txt", "text/plain", "u"); got != "[log.txt](u)" {
		t.Errorf("got %s", got)
	}
	for size, want := range map[int64]string{12: "12 B", 2048: "2.0 KB", 3 * 1024 * 1024: "3.0 MB"} {
		if got := humanSize(size); got != want {
			t.Errorf("%d: got %s", size, got)
		}
	}
}
