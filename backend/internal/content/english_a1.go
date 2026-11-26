package content

import "ortalyq/backend/internal/modules/exercises"

const (
	instant = exercises.CheckInstant
	after   = exercises.CheckAfter
)

// Section titles of every lesson in the course.
const (
	SecIntro    = "Знакомство с темой"
	SecPractice = "Практика"
	SecTalk     = "Разговор"
)

// EnglishA1 is «English A1 — Первые разговоры»: three conversation lessons for adult beginners.
// Tasks and options are in English, the teacher's notes are in Russian. Every gap has one phrase and
// one unambiguous answer, every test question has one correct option.
var EnglishA1 = Course{
	Key:   "english-a1-first-conversations",
	Title: "English A1 — Первые разговоры",
	Description: "Авторский разговорный курс для взрослых, которые начинают с нуля. Три урока: поздороваться и назвать имя, " +
		"рассказать, откуда ты, представить друга. В каждом уроке — диалог, полезные фразы и разговорное задание, " +
		"тест, три упражнения «Слова и пропуски» и два задания на дом.",
	CoverColor: "#16C784",
	Lessons:    []Lesson{lessonHello, lessonFrom, lessonFriend},
}

func attrs(grammar, vocab, functions []string) Attrs {
	return Attrs{Language: "English", Age: "Adults", Level: "A1", LessonType: "Conversation", DurationMin: 45,
		Skills: []string{"Speaking", "Listening"}, Grammar: grammar, Vocabulary: vocab, Functions: functions,
		Tags: []string{"First conversations"}}
}

var lessonHello = Lesson{
	Key:   "hello",
	Title: "Lesson 1. Hello!",
	Attrs: attrs([]string{"to be: I am", "My name is"}, []string{"Greetings"}, []string{"Greeting", "Introducing yourself", "Asking for a name"}),
	Guide: Guide{
		Goal: "Ученик здоровается, называет своё имя, спрашивает имя собеседника и прощается.",
		Dialogue: []Line{
			{"Aruzhan", "Hello! I'm Aruzhan."},
			{"Ben", "Hi, Aruzhan! My name is Ben."},
			{"Aruzhan", "Nice to meet you, Ben."},
			{"Ben", "Nice to meet you too. Sorry, can you spell your name?"},
			{"Aruzhan", "Sure. A-R-U-Z-H-A-N."},
			{"Ben", "Thank you! See you later, Aruzhan."},
			{"Aruzhan", "Bye, Ben!"},
		},
		Phrases: []Phrase{
			{"Hello! / Hi!", "Здравствуйте! / Привет!"},
			{"I'm … / My name is …", "Я … / Меня зовут …"},
			{"What's your name?", "Как тебя зовут?"},
			{"Nice to meet you.", "Приятно познакомиться."},
			{"Nice to meet you too.", "Мне тоже приятно познакомиться."},
			{"Can you spell your name?", "Можешь продиктовать имя по буквам?"},
			{"See you later! / Bye!", "До встречи! / Пока!"},
		},
		Steps: []Step{
			{SecIntro, "Поздоровайтесь с учеником и назовите себя: «Hi! I'm …». Прочитайте диалог вслух, затем по ролям. " +
				"Разберите фразы из таблицы: ученик повторяет каждую за вами. Выполните пропуск «My name ___ Ben.»."},
			{SecPractice, "Два пропуска на I am и What's your name?. Если ученик ошибся, попросите произнести всю фразу целиком."},
			{SecTalk, "Тест «Choose the best reply»: ученик выбирает ответ и сразу читает мини-диалог вслух. Затем разговорное задание."},
		},
		Speaking: "Ролевая игра «Первый день в языковой школе». Вы играете нового студента, ученик — себя, потом поменяйтесь. " +
			"Ученик работает по карточке «Speaking task». Следите, чтобы он говорил полными фразами, и в конце назовите одну фразу, которую стоит повторить.",
		SpeakingCard: []string{
			"Role play: your first day at a language school. Your teacher is a new student.",
			"1. Say hello.",
			"2. Say your name.",
			"3. Ask your partner's name.",
			"4. Ask your partner to spell their name.",
			"5. Say goodbye.",
			"Then swap roles.",
		},
		Homework: "Пропуск «Nice to ___ you» и тест «Choose the correct sentence». Ученик отправляет работу на проверку, " +
			"вы принимаете её или возвращаете на доработку на вкладке «Домашняя работа».",
	},
	Sections: []Section{
		{SecIntro, []Exercise{
			Gap("Complete the introduction", "Hi, Aruzhan! My name ___ Ben.", "is", instant),
		}},
		{SecPractice, []Exercise{
			Gap("Say your name", "Hello! I ___ Aruzhan.", "am", instant),
			Gap("Ask for a name", "— What's your ___? — My name is Ben.", "name", instant),
		}},
		{SecTalk, []Exercise{
			Test("Choose the best reply", instant,
				Q("— Hello! I'm Ben.", 1, "Goodbye, Ben.", "Hi, Ben! I'm Aruzhan.", "Yes, it is."),
				Q("— Nice to meet you.", 0, "Nice to meet you too.", "My name is you.", "See you yesterday."),
				Q("Which question asks for a person's name?", 2, "How are you?", "Where are you from?", "What's your name?"),
			),
		}},
	},
	Homework: []Exercise{
		Gap("Homework: complete the reply", "— Nice to ___ you, Ben. — Nice to meet you too.", "meet", after),
		Test("Homework: choose the correct sentence", after,
			Q("Choose the correct sentence.", 1, "I is Ben.", "I am Ben.", "I are Ben."),
			Q("Choose the correct sentence.", 2, "My name are Dana.", "My name am Dana.", "My name is Dana."),
			Q("Which word is a greeting?", 0, "Hi", "Bye", "Thanks"),
		),
	},
}

var lessonFrom = Lesson{
	Key:   "where-are-you-from",
	Title: "Lesson 2. Where are you from?",
	Attrs: attrs([]string{"to be: I am, you are", "from"}, []string{"Countries", "Cities"}, []string{"Saying where you are from", "Asking where someone is from"}),
	Guide: Guide{
		Goal: "Ученик говорит, из какой он страны и города, и спрашивает об этом собеседника.",
		Dialogue: []Line{
			{"Dana", "Hi, I'm Dana. What's your name?"},
			{"Marco", "Hello, Dana. I'm Marco."},
			{"Dana", "Where are you from, Marco?"},
			{"Marco", "I'm from Italy. I'm from Rome. And you?"},
			{"Dana", "I'm from Kazakhstan. I'm from Almaty."},
			{"Marco", "Almaty? Is it a big city?"},
			{"Dana", "Yes, it is. It's beautiful!"},
		},
		Phrases: []Phrase{
			{"Where are you from?", "Откуда ты?"},
			{"I'm from Kazakhstan.", "Я из Казахстана."},
			{"I'm from Almaty.", "Я из Алматы."},
			{"And you?", "А ты?"},
			{"Is it a big city?", "Это большой город?"},
			{"Yes, it is. / No, it isn't.", "Да. / Нет."},
			{"It's beautiful!", "Он красивый!"},
		},
		Steps: []Step{
			{SecIntro, "Повторите приветствие из урока 1: «Hi! I'm … What's your name?». Прочитайте диалог по ролям, " +
				"разберите фразы. Обратите внимание: from ставится перед страной и городом. Выполните пропуск «I ___ from Almaty.»."},
			{SecPractice, "Пропуски на вопрос Where are you from? Попросите ученика задать вопрос вам и ответить на ваш."},
			{SecTalk, "Тест «Choose the best reply», затем разговорное задание."},
		},
		Speaking: "Игра «Международная конференция». Ученик знакомится с вами от лица каждого героя с карточки «Speaking task», " +
			"вы отвечаете и задаёте встречные вопросы. В конце ученик говорит о себе: «I'm from …».",
		SpeakingCard: []string{
			"Game: an international conference. Be each person and meet your teacher.",
			"• Ana — Spain, Madrid",
			"• Kenji — Japan, Tokyo",
			"• Aigerim — Kazakhstan, Astana",
			"1. Say hello and your name.",
			"2. Say where you are from.",
			"3. Ask: Where are you from?",
			"4. Ask: Is it a big city?",
			"At the end, talk about you: I'm from …",
		},
		Homework: "Пропуск «Yes, it ___» и тест «Choose the correct answer». Проверьте отправленную работу на вкладке «Домашняя работа».",
	},
	Sections: []Section{
		{SecIntro, []Exercise{
			Gap("Say where you are from", "I ___ from Almaty.", "am", instant),
		}},
		{SecPractice, []Exercise{
			Gap("Ask the question", "— Where ___ you from? — I'm from Italy.", "are", instant),
			Gap("Finish the question", "— Where are you ___? — I'm from Kazakhstan.", "from", instant),
		}},
		{SecTalk, []Exercise{
			Test("Choose the best reply", instant,
				Q("— Where are you from?", 0, "I'm from Kazakhstan.", "I'm Dana.", "Nice to meet you."),
				Q("— I'm from Rome. And you?", 2, "Yes, I am.", "My name is Rome.", "I'm from Almaty."),
				Q("— Is Almaty a big city?", 1, "Yes, I am.", "Yes, it is.", "Yes, you are."),
			),
		}},
	},
	Homework: []Exercise{
		Gap("Homework: answer the question", "— Is Almaty a big city? — Yes, it ___.", "is", after),
		Test("Homework: choose the correct answer", after,
			Q("Choose the correct question.", 0, "Where are you from?", "Where you are from?", "Where is you from?"),
			Q("Which one is a country?", 1, "Almaty", "Kazakhstan", "Rome"),
			Q("— Are you from Almaty? — No, I ___ .", 0, "am not", "isn't", "aren't"),
		),
	},
}

var lessonFriend = Lesson{
	Key:   "meet-my-friend",
	Title: "Lesson 3. Meet my friend",
	Attrs: attrs([]string{"to be: he is, she is", "his, her"}, []string{"People", "Jobs"}, []string{"Introducing someone", "Talking about a person"}),
	Guide: Guide{
		Goal: "Ученик представляет другого человека и рассказывает о нём двумя простыми предложениями: откуда он и кто он.",
		Dialogue: []Line{
			{"Ali", "Hi, Sara! This is my friend Tom."},
			{"Sara", "Hi, Tom! Nice to meet you."},
			{"Tom", "Nice to meet you too."},
			{"Ali", "Tom is from London. He is a student."},
			{"Sara", "Great! I'm a student too."},
			{"Ali", "And this is my sister Lina. She is from Astana. She is a teacher."},
			{"Tom", "Hello, Lina!"},
		},
		Phrases: []Phrase{
			{"This is my friend …", "Это мой друг / моя подруга …"},
			{"This is my sister / brother …", "Это моя сестра / мой брат …"},
			{"He is from … / She is from …", "Он из … / Она из …"},
			{"He is a student. / She is a teacher.", "Он студент. / Она учитель."},
			{"His name is … / Her name is …", "Его зовут … / Её зовут …"},
			{"I'm a student too.", "Я тоже студент."},
		},
		Steps: []Step{
			{SecIntro, "Повторите «Where are you from?» из урока 2. Покажите фото знакомого человека и скажите: " +
				"«This is my friend … He is from … He is a …». Прочитайте диалог по ролям, разберите фразы. " +
				"Отдельно проговорите he / she и his / her. Выполните пропуск «This ___ my friend Tom.»."},
			{SecPractice, "Пропуски на he и she. Спросите, почему в одном случае he, а в другом she."},
			{SecTalk, "Тест «Choose the best answer», затем разговорное задание."},
		},
		Speaking: "«Представь друга». Попросите ученика заранее выбрать друга или родственника (можно фото) и представить его по карточке «Speaking task». " +
			"Затем вы представляете своего «друга», а ученик реагирует: «Hi, …! Nice to meet you.» и задаёт вопрос «Where are you from?».",
		SpeakingCard: []string{
			"Introduce a friend or a family member. You can show a photo.",
			"1. Say: This is my friend … / This is my sister … / This is my brother …",
			"2. Say two sentences about him or her: He/She is from … He/She is a …",
			"Then listen to your teacher's friend. Say: Hi, …! Nice to meet you. Ask: Where are you from?",
		},
		Homework: "Пропуск «___ name is Ali» и тест «Choose the correct word». Проверьте работу на вкладке «Домашняя работа».",
	},
	Sections: []Section{
		{SecIntro, []Exercise{
			Gap("Introduce a friend", "This ___ my friend Tom.", "is", instant),
		}},
		{SecPractice, []Exercise{
			Gap("Talk about Tom", "This is my friend Tom. He is from London. ___ is a student.", "he", instant),
			Gap("Talk about Lina", "This is my sister Lina. ___ is a teacher.", "she", instant),
		}},
		{SecTalk, []Exercise{
			Test("Choose the best answer", instant,
				Q("Choose the correct sentence to introduce a friend.", 0, "This is my friend Tom.", "This are my friend Tom.", "He my friend Tom."),
				Q("— This is my friend Tom.", 1, "Yes, I am Tom.", "Hi, Tom! Nice to meet you.", "Goodbye, Tom!"),
				Q("Which two sentences tell about Lina?", 2, "He is from Astana. He are a teacher.", "She from Astana. She a teacher.", "She is from Astana. She is a teacher."),
			),
		}},
	},
	Homework: []Exercise{
		Gap("Homework: his or her?", "This is my brother. ___ name is Ali.", "his", after),
		Test("Homework: choose the correct word", after,
			Q("My brother ___ from Almaty.", 0, "is", "am", "are"),
			Q("This is my sister. ___ name is Aigerim.", 1, "His", "Her", "She"),
			Q("Which sentence introduces a friend?", 2, "My name is Dana.", "Where is Dana from?", "This is my friend Dana."),
		),
	},
}
