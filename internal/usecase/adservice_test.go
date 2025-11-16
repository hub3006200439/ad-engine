package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Мок репозитория. Подмена AdRepository.
//   - корректную передачу входных параметров
//   - возврат ошибок
//   - корректное делегирование вызовов репозиторию
type mockAdRepository struct {
	mock.Mock
}

func (m *mockAdRepository) RequestAd(ctx context.Context, req AdRequest) (*AdResponse, error) {
	args := m.Called(ctx, req)
	if resp, ok := args.Get(0).(*AdResponse); ok {
		return resp, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockAdRepository) TrackClick(ctx context.Context, token string) (string, error) {
	args := m.Called(ctx, token)
	return args.String(0), args.Error(1)
}

func (m *mockAdRepository) TrackView(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

//     Тест RequestAd

func TestAdService_RequestAd_Success(t *testing.T) {
	// Проверка:
	//   - AdService вызывает репозиторий с теми же параметрами
	//   - репозиторий возвращает готовый AdResponse
	//   - AdService возвращает его без изменений

	ctx := context.Background()
	mockRepo := &mockAdRepository{}
	svc := NewAdService(mockRepo, nil)

	req := AdRequest{
		UserID:    uuid.NewString(),
		SessionID: uuid.NewString(),
		Lang:      "en",
		Country:   "US",
		Category:  "tech",
		Placement: "pre-roll",
		RequestID: "req-123",
	}

	expected := &AdResponse{
		Token:      "token-123",
		CreativeID: uuid.New(),
		Duration:   30,
		VideoURL:   "https://cdn.example.com/video.mp4",
		ClickURL:   "/ad/click/token-123",
		ViewURL:    "/ad/view/token-123",
	}

	// Устанавливаем ожидание — "мокнутый" репозиторий должен быть вызван:
	mockRepo.On("RequestAd", mock.Anything, req).Return(expected, nil)

	resp, err := svc.RequestAd(ctx, req)

	// - ошибки нет
	// - ответ совпадает с тем, что вернул репозиторий
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, expected.Token, resp.Token)

	mockRepo.AssertExpectations(t)
}

func TestAdService_RequestAd_Error(t *testing.T) {
	// Проверяем, что в случае ошибки в репозитории,
	// AdService честно её пробрасывает наверх.

	ctx := context.Background()
	mockRepo := &mockAdRepository{}
	svc := NewAdService(mockRepo, nil)

	req := AdRequest{
		UserID:    uuid.NewString(),
		SessionID: uuid.NewString(),
		Lang:      "en",
		Country:   "US",
		Category:  "tech",
		Placement: "pre-roll",
	}

	mockErr := errors.New("db error")

	mockRepo.On("RequestAd", mock.Anything, req).Return((*AdResponse)(nil), mockErr)

	resp, err := svc.RequestAd(ctx, req)

	require.Error(t, err)
	require.Nil(t, resp)
	assert.ErrorIs(t, err, mockErr)

	mockRepo.AssertExpectations(t)
}

//
// ------------------------
//      TrackClick
// ------------------------
//

func TestAdService_TrackClick_Success(t *testing.T) {
	// Успешный клик должен вернуть landing_url
	// и корректно делегировать вызов репозиторию.

	ctx := context.Background()
	mockRepo := &mockAdRepository{}
	svc := NewAdService(mockRepo, nil)

	token := "click-token-123"
	landing := "https://example.com/landing"

	mockRepo.On("TrackClick", mock.Anything, token).Return(landing, nil)

	got, err := svc.TrackClick(ctx, token)

	require.NoError(t, err)
	assert.Equal(t, landing, got)

	mockRepo.AssertExpectations(t)
}

func TestAdService_TrackClick_Error(t *testing.T) {
	// Ошибка репозитория должна пробрасываться наверх.

	ctx := context.Background()
	mockRepo := &mockAdRepository{}
	svc := NewAdService(mockRepo, nil)

	token := "click-token-123"
	mockErr := errors.New("db error")

	mockRepo.On("TrackClick", mock.Anything, token).Return("", mockErr)

	got, err := svc.TrackClick(ctx, token)

	require.Error(t, err)
	assert.Empty(t, got)
	assert.ErrorIs(t, err, mockErr)

	mockRepo.AssertExpectations(t)
}

//
// ------------------------
//        TrackView
// ------------------------
//

func TestAdService_TrackView_Success(t *testing.T) {
	// TrackView — просто обёртка, проверяем корректную передачу ошибок.

	ctx := context.Background()
	mockRepo := &mockAdRepository{}
	svc := NewAdService(mockRepo, nil)

	token := "view-token-123"

	mockRepo.On("TrackView", mock.Anything, token).Return(nil)

	err := svc.TrackView(ctx, token)

	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestAdService_TrackView_Error(t *testing.T) {
	// Ошибка репозитория должна пробрасываться.

	ctx := context.Background()
	mockRepo := &mockAdRepository{}
	svc := NewAdService(mockRepo, nil)

	token := "view-token-123"
	mockErr := errors.New("db error")

	mockRepo.On("TrackView", mock.Anything, token).Return(mockErr)

	err := svc.TrackView(ctx, token)

	require.Error(t, err)
	assert.ErrorIs(t, err, mockErr)

	mockRepo.AssertExpectations(t)
}
