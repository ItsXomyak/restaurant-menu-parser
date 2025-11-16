package parser

import (
    "context"
    "fmt"
    "log/slog"
    "strconv"
    "strings"

    "golang.org/x/oauth2/google"
    "google.golang.org/api/option"
    "google.golang.org/api/sheets/v4"

    "github.com/ItsXomyak/restaurant-menu-parser/internal/domain/models"
    "github.com/ItsXomyak/restaurant-menu-parser/internal/domain/types"
    "github.com/ItsXomyak/restaurant-menu-parser/pkg/logger"
)

type GoogleSheetParser struct {
    sheetsService *sheets.Service
}

func NewGoogleSheetParser(ctx context.Context, serviceAccountJSON []byte) (*GoogleSheetParser, error) {
    config, err := google.JWTConfigFromJSON(serviceAccountJSON, sheets.SpreadsheetsReadonlyScope)
    if err != nil {
        return nil, fmt.Errorf("не удалось прочитать конфиг service account: %w", err)
    }

    client := config.Client(ctx)
    service, err := sheets.NewService(ctx, option.WithHTTPClient(client))
    if err != nil {
        return nil, fmt.Errorf("не удалось создать Google Sheets service: %w", err)
    }

    return &GoogleSheetParser{
        sheetsService: service,
    }, nil
}

func (p *GoogleSheetParser) ParseMenu(ctx context.Context, spreadsheetID string) (*models.ParsedMenuData, error) {
    l := logger.LoggerFromContext(ctx)
    parsedData := &models.ParsedMenuData{
        Products:        make(map[string]*models.Product),
        AttributeGroups: make(map[string]*models.AttributeGroup),
        Attributes:      make(map[string]*models.Attribute),
    }
    spreadsheet, err := p.sheetsService.Spreadsheets.Get(spreadsheetID).Fields("sheets.properties.title").Context(ctx).Do()
    if err != nil {
        return nil, fmt.Errorf("%w: не удалось получить листы таблицы: %w", types.ErrSpreadsheetParse, err)
    }
    for _, sheet := range spreadsheet.Sheets {
        sheetName := sheet.Properties.Title
        if sheetName == "Прочее" || sheetName == "Специальные предложения Glovo" {
            l.Info("Пропускаем лист", "sheet", sheetName)
            continue
        }
        readRange := fmt.Sprintf("'%s'!A:L", sheetName)
        resp, err := p.sheetsService.Spreadsheets.Values.Get(spreadsheetID, readRange).Context(ctx).Do()
        if err != nil {
            l.Warn("Не удалось прочитать лист, пропускаем", "sheet", sheetName, slog.Any("error", err))
            continue
        }

        if len(resp.Values) == 0 {
            continue
        }
        var currentProduct *models.Product
        var currentAttributeGroup *models.AttributeGroup

        for i, row := range resp.Values {
            if i == 0 {
                continue
            }

            sheetRow := parseRowToSheetRow(row)
            if sheetRow.ProductID != "" {
                product := &models.Product{
                    ExtID:             sheetRow.ProductID,
                    Name:              sheetRow.ProductName,
                    Description:       sheetRow.Description,
                    Price:             sheetRow.Price,
                    IsCombo:           sheetRow.IsCombo,
                    Status:            string(models.ProductStatusAvailable),
                    AttributeGroupIDs: []string{},
                }
                parsedData.Products[product.ExtID] = product
                currentProduct = product
            }
            if sheetRow.AttributeGroupID != "" {
                if _, ok := parsedData.AttributeGroups[sheetRow.AttributeGroupID]; !ok {
                    attrGroup := &models.AttributeGroup{
                        ID:         sheetRow.AttributeGroupID,
                        Name:       sheetRow.AttributeGroupName,
                        MinSelect:  sheetRow.MinSelect,
                        MaxSelect:  sheetRow.MaxSelect,
                        IsRequired: sheetRow.MinSelect >= 1,
                        IsMultiple: sheetRow.MaxSelect > 1,
                    }
                    parsedData.AttributeGroups[attrGroup.ID] = attrGroup
                    currentAttributeGroup = attrGroup
                } else {
                    currentAttributeGroup = parsedData.AttributeGroups[sheetRow.AttributeGroupID]
                }

                if currentProduct != nil {
                    currentProduct.AttributeGroupIDs = appendIfMissing(currentProduct.AttributeGroupIDs, sheetRow.AttributeGroupID)
                }
            }
            if sheetRow.AttributeID != "" && currentAttributeGroup != nil {
                if _, ok := parsedData.Attributes[sheetRow.AttributeID]; !ok {

                    attr := &models.Attribute{
                        ID:      sheetRow.AttributeID,
                        GroupID: currentAttributeGroup.ID,
                        Name:    sheetRow.AttributeName,
                        Price:   sheetRow.AttributePrice,
                        Min:     sheetRow.AttributeMin,
                        Max:     sheetRow.AttributeMax,
                    }

                    parsedData.Attributes[attr.ID] = attr
                }
            }
        }
    }
    return parsedData, nil
}

func parseRowToSheetRow(row []interface{}) models.SheetRow {
    return models.SheetRow{
        ProductID: getString(row, 0),
        ProductName: getString(row, 1),
        IsCombo: getBool(row, 2),
        Price: getFloat(row, 3),
        Description: getString(row, 4),
        AttributeGroupID: getString(row, 5),
        AttributeGroupName: getString(row, 6),
        MinSelect: getInt(row, 7),
        MaxSelect: getInt(row, 8),
        AttributeID: getString(row, 9),
        AttributeName: getString(row, 10),
        AttributeMin: getInt(row, 11),
        AttributeMax: getInt(row, 12),
        AttributePrice: getFloat(row, 13),
    }
}


func getString(row []interface{}, index int) string {
	if index < len(row) {
		if val, ok := row[index].(string); ok {
			return val
		}
	}
	return ""
}

func getInt(row []interface{}, index int) int {
	if index < len(row) {
		if val, ok := row[index].(string); ok {
			i, _ := strconv.Atoi(val)
			return i
		}
	}
	return 0
}

func getFloat(row []interface{}, index int) float64 {
    if index < len(row) {
        if val, ok := row[index].(string); ok {
            val = strings.ReplaceAll(val, " ", "")
            val = strings.ReplaceAll(val, ",", ".")
            f, _ := strconv.ParseFloat(val, 64)
            return f
        }
    }
    return 0.0
}

func getBool(row []interface{}, index int) bool {
    if index < len(row) {
        if val, ok := row[index].(string); ok {
            return strings.TrimSpace(val) == "TRUE"
        }
    }
    return false
}

func appendIfMissing(slice []string, s string) []string {
	for _, ele := range slice {
		if ele == s {
			return slice
		}
	}
	return append(slice, s)
}
