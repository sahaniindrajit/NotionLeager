package charts

import (
	"os"
	"path/filepath"

	"notionLeager/expense"

	"github.com/wcharczuk/go-chart/v2"
)

func GenerateCategoryPie(
	data []expense.CategoryTotal,
	outputPath string,
) error {

	var values []chart.Value

	for _, c := range data {
		values = append(values, chart.Value{
			Value: c.Amount,
			Label: c.Category,
		})
	}

	pie := chart.PieChart{
		Width:  700,
		Height: 700,
		Values: values,
	}

	dir := filepath.Dir(outputPath)
	_ = os.MkdirAll(dir, 0755)

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return pie.Render(chart.PNG, file)
}
