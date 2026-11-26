package exercises

// pdfKind is «PDF-документ»: a PDF file shown in a lesson section for reading (a worksheet, a text, a rule). Like
// «Текст» it has no answer and no grade: no attempt, left out of points, progress and snapshots (see Gradable). The
// version keeps the title and the file (file_id); there are no questions. The service checks that the file was
// uploaded by the teacher (or is the block's current file).
type pdfKind struct{}

func (pdfKind) Validate(req SaveRequest) map[string]string {
	errs := map[string]string{}
	if req.FileID == nil {
		errs["file_id"] = "Выберите PDF-файл"
	}
	if len(req.Questions) != 0 {
		errs["questions"] = "У PDF-документа нет вопросов"
	}
	if req.Settings.TimerEnabled || req.Settings.ScoringEnabled {
		errs["settings"] = "У PDF-документа нет таймера и баллов"
	}
	return errs
}

func (pdfKind) Grade(Question, Response) AnswerResult { return Unanswered }

func (pdfKind) Sanitize(q Question) Question { return q }
