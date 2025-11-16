package types

import "errors"

var (
	ErrNotFound      = errors.New("сущность не найдена")
	ErrConflict      = errors.New("конфликт, сущность уже существует")
	ErrInternal      = errors.New("внутренняя ошибка")
	ErrInvalidInput  = errors.New("некорректные входные данные")
	ErrBadRequest	 = errors.New("ошибка в запросе")

	ErrTaskCreateFail   = errors.New("не удалось создать задачу")
	ErrTaskPublishFail  = errors.New("не удалось опубликовать задачу")
	ErrMenuSaveFail     = errors.New("не удалось сохранить меню")
	ErrSpreadsheetParse = errors.New("ошибка парсинга таблицы")
	ErrEventPublishFail = errors.New("не удалось опубликовать событие")
	ErrTaskGetFail      = errors.New("не удалось получить задачу из БД")
	ErrTaskUpdateFail   = errors.New("не удалось обновить задачу в БД")

	ErrMenuGetFail       = errors.New("не удалось получить меню")
	ErrMenuUpdateFail    = errors.New("не удалось обновить продукт в меню")
	ErrAuditCreateFail = errors.New("не удалось создать лог аудита")
)