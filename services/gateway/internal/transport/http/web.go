package httptransport

import (
	"html/template"
	"log"
	"net/http"
)

type WebHandler struct {
	page *template.Template
}

type webPageData struct {
	TelegramBotUsername string
}

func NewWebHandler(
	telegramBotUsername string,
) *WebHandler {
	page := template.Must(
		template.New("index").
			Parse(indexHTML),
	)

	return &WebHandler{
		page: page,
	}
}

func (handler *WebHandler) Index(
	w http.ResponseWriter,
	r *http.Request,
	telegramBotUsername string,
) {
	w.Header().Set(
		"Content-Type",
		"text/html; charset=utf-8",
	)

	err := handler.page.Execute(
		w,
		webPageData{
			TelegramBotUsername: telegramBotUsername,
		},
	)

	if err != nil {
		log.Printf(
			"render web page: %v",
			err,
		)
	}
}

const indexHTML = `
<!doctype html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta
        name="viewport"
        content="width=device-width, initial-scale=1.0"
    >

    <title>MovieTracker</title>

    <style>
        * {
            box-sizing: border-box;
        }

        body {
            margin: 0;
            font-family:
                -apple-system,
                BlinkMacSystemFont,
                "Segoe UI",
                sans-serif;
            background: #f4f5f7;
            color: #1f2937;
        }

        .page {
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 24px;
        }

        .card {
            width: 100%;
            max-width: 430px;
            background: white;
            border-radius: 16px;
            padding: 32px;
            box-shadow:
                0 12px 35px
                rgba(0, 0, 0, 0.08);
        }

        h1 {
            margin-top: 0;
            margin-bottom: 8px;
        }

        .subtitle {
            margin-top: 0;
            margin-bottom: 28px;
            color: #6b7280;
        }

        .field {
            margin-bottom: 16px;
        }

        label {
            display: block;
            margin-bottom: 6px;
            font-size: 14px;
            font-weight: 600;
        }

        input {
            width: 100%;
            padding: 11px 12px;
            border: 1px solid #d1d5db;
            border-radius: 8px;
            font-size: 15px;
        }

        input:focus {
            outline: none;
            border-color: #111827;
        }

        button,
        .telegram-link {
            width: 100%;
            display: block;
            border: 0;
            border-radius: 8px;
            padding: 12px 16px;
            font-size: 15px;
            cursor: pointer;
            text-align: center;
            text-decoration: none;
        }

        button {
            background: #111827;
            color: white;
        }

        button.secondary {
            margin-top: 10px;
            background: #e5e7eb;
            color: #111827;
        }

        button.telegram {
            background: #229ed9;
        }

        button:disabled {
            opacity: 0.6;
            cursor: wait;
        }

        .switch {
            margin-top: 20px;
            text-align: center;
            color: #6b7280;
            font-size: 14px;
        }

        .switch button {
            display: inline;
            width: auto;
            padding: 0;
            background: transparent;
            color: #111827;
            text-decoration: underline;
        }

        .message {
            display: none;
            margin-top: 18px;
            padding: 12px;
            border-radius: 8px;
            font-size: 14px;
            white-space: pre-line;
        }

        .message.error {
            display: block;
            background: #fee2e2;
            color: #991b1b;
        }

        .message.success {
            display: block;
            background: #dcfce7;
            color: #166534;
        }

        .account {
            display: none;
        }

        .account-email {
            margin-bottom: 24px;
            font-weight: 600;
        }

        .link-code {
            margin-top: 20px;
            padding: 16px;
            background: #f3f4f6;
            border-radius: 8px;
        }

        .link-code code {
            font-size: 18px;
            font-weight: 700;
        }

        .hint {
            margin-top: 8px;
            font-size: 13px;
            color: #6b7280;
        }
    </style>
</head>

<body>

<div class="page">
    <main class="card">

        <section id="authSection">

            <h1>MovieTracker</h1>

            <p
                id="authSubtitle"
                class="subtitle"
            >
                Войди в свой аккаунт
            </p>

            <form id="authForm">

                <div class="field">
                    <label for="email">
                        Email
                    </label>

                    <input
                        id="email"
                        type="email"
                        autocomplete="email"
                        required
                    >
                </div>

                <div class="field">
                    <label for="password">
                        Пароль
                    </label>

                    <input
                        id="password"
                        type="password"
                        autocomplete="current-password"
                        minlength="8"
                        required
                    >
                </div>

                <button
                    id="submitButton"
                    type="submit"
                >
                    Войти
                </button>

            </form>

            <div class="switch">
                <span id="switchText">
                    Нет аккаунта?
                </span>

                <button
                    id="switchMode"
                    type="button"
                >
                    Зарегистрироваться
                </button>
            </div>

            <div
                id="authMessage"
                class="message"
            ></div>

        </section>


        <section
            id="accountSection"
            class="account"
        >

            <h1>MovieTracker</h1>

            <p class="subtitle">
                Аккаунт
            </p>

            <div
                id="accountEmail"
                class="account-email"
            ></div>

            <button
                id="connectTelegram"
                class="telegram"
                type="button"
            >
                Подключить Telegram
            </button>

            <div
                id="telegramResult"
                class="link-code"
                style="display:none"
            >
                <div>
                    Код привязки:
                </div>

                <p>
                    <code id="telegramCode"></code>
                </p>

                <div class="hint">
                    Если Telegram не открылся автоматически,
                    отправь боту команду
                    <strong id="telegramCommand"></strong>
                </div>
            </div>

            <button
                id="logout"
                class="secondary"
                type="button"
            >
                Выйти
            </button>

            <div
                id="accountMessage"
                class="message"
            ></div>

        </section>

    </main>
</div>

<script>
    const telegramBotUsername =
        {{ printf "%q" .TelegramBotUsername }};

    const authSection =
        document.getElementById("authSection");

    const accountSection =
        document.getElementById("accountSection");

    const authForm =
        document.getElementById("authForm");

    const emailInput =
        document.getElementById("email");

    const passwordInput =
        document.getElementById("password");

    const submitButton =
        document.getElementById("submitButton");

    const switchModeButton =
        document.getElementById("switchMode");

    const switchText =
        document.getElementById("switchText");

    const authSubtitle =
        document.getElementById("authSubtitle");

    const authMessage =
        document.getElementById("authMessage");

    const accountMessage =
        document.getElementById("accountMessage");

    const accountEmail =
        document.getElementById("accountEmail");

    const connectTelegramButton =
        document.getElementById("connectTelegram");

    const telegramResult =
        document.getElementById("telegramResult");

    const telegramCode =
        document.getElementById("telegramCode");

    const telegramCommand =
        document.getElementById("telegramCommand");

    const logoutButton =
        document.getElementById("logout");

    let mode = "login";


    function setMessage(element, text, type) {
        element.textContent = text;
        element.className =
            "message " + type;
    }


    function clearMessage(element) {
        element.textContent = "";
        element.className = "message";
    }


    function showAccount(email) {
        authSection.style.display = "none";
        accountSection.style.display = "block";

        accountEmail.textContent = email;

        telegramResult.style.display = "none";

        clearMessage(accountMessage);
    }


    function showAuth() {
        accountSection.style.display = "none";
        authSection.style.display = "block";

        clearMessage(authMessage);
    }


    function getToken() {
        return sessionStorage.getItem(
            "movietracker_access_token"
        );
    }


    function saveSession(
        token,
        email
    ) {
        sessionStorage.setItem(
            "movietracker_access_token",
            token
        );

        sessionStorage.setItem(
            "movietracker_email",
            email
        );
    }


    function clearSession() {
        sessionStorage.removeItem(
            "movietracker_access_token"
        );

        sessionStorage.removeItem(
            "movietracker_email"
        );
    }


    async function requestJSON(
        url,
        options
    ) {
        const response =
            await fetch(
                url,
                options
            );

        let data = null;

        try {
            data = await response.json();
        } catch (_) {
        }

        if (!response.ok) {
            const message =
                data?.error?.message ||
                "HTTP " + response.status;

            throw new Error(message);
        }

        return data;
    }


    async function login(
        email,
        password
    ) {
        const data =
            await requestJSON(
                "/auth/login",
                {
                    method: "POST",

                    headers: {
                        "Content-Type":
                            "application/json"
                    },

                    body: JSON.stringify({
                        email,
                        password
                    })
                }
            );

        if (!data.access_token) {
            throw new Error(
                "Auth service did not return access token"
            );
        }

        saveSession(
            data.access_token,
            email
        );

        showAccount(email);
    }


    async function register(
        email,
        password
    ) {
        await requestJSON(
            "/auth/register",
            {
                method: "POST",

                headers: {
                    "Content-Type":
                        "application/json"
                },

                body: JSON.stringify({
                    email,
                    password
                })
            }
        );

        await login(
            email,
            password
        );
    }


    authForm.addEventListener(
        "submit",
        async function(event) {
            event.preventDefault();

            clearMessage(authMessage);

            const email =
                emailInput.value.trim();

            const password =
                passwordInput.value;

            submitButton.disabled = true;

            try {
                if (mode === "login") {
                    await login(
                        email,
                        password
                    );
                } else {
                    await register(
                        email,
                        password
                    );
                }
            } catch (error) {
                setMessage(
                    authMessage,
                    error.message,
                    "error"
                );
            } finally {
                submitButton.disabled = false;
            }
        }
    );


    switchModeButton.addEventListener(
        "click",
        function() {
            clearMessage(authMessage);

            if (mode === "login") {
                mode = "register";

                authSubtitle.textContent =
                    "Создай аккаунт";

                submitButton.textContent =
                    "Зарегистрироваться";

                switchText.textContent =
                    "Уже есть аккаунт?";

                switchModeButton.textContent =
                    "Войти";

                passwordInput.autocomplete =
                    "new-password";
            } else {
                mode = "login";

                authSubtitle.textContent =
                    "Войди в свой аккаунт";

                submitButton.textContent =
                    "Войти";

                switchText.textContent =
                    "Нет аккаунта?";

                switchModeButton.textContent =
                    "Зарегистрироваться";

                passwordInput.autocomplete =
                    "current-password";
            }
        }
    );


    connectTelegramButton.addEventListener(
        "click",
        async function() {
            clearMessage(accountMessage);

            const token = getToken();

            if (!token) {
                clearSession();
                showAuth();
                return;
            }

            connectTelegramButton.disabled = true;

            try {
                const data =
                    await requestJSON(
                        "/auth/telegram/link-code",
                        {
                            method: "POST",

                            headers: {
                                "Authorization":
                                    "Bearer " + token
                            }
                        }
                    );

                if (!data.code) {
                    throw new Error(
                        "Auth service did not return link code"
                    );
                }

                telegramCode.textContent =
                    data.code;

                telegramCommand.textContent =
                    "/link " + data.code;

                telegramResult.style.display =
                    "block";

                const startParameter =
                    "link_" + data.code;

                const telegramURL =
                    "https://t.me/" +
                    telegramBotUsername +
                    "?start=" +
                    encodeURIComponent(
                        startParameter
                    );

                window.location.href =
                    telegramURL;

            } catch (error) {
                if (
                    error.message
                        .toLowerCase()
                        .includes("token")
                ) {
                    clearSession();
                    showAuth();
                    return;
                }

                setMessage(
                    accountMessage,
                    error.message,
                    "error"
                );

            } finally {
                connectTelegramButton.disabled =
                    false;
            }
        }
    );


    logoutButton.addEventListener(
        "click",
        function() {
            clearSession();
            showAuth();
        }
    );


    const existingToken =
        getToken();

    const existingEmail =
        sessionStorage.getItem(
            "movietracker_email"
        );

    if (
        existingToken &&
        existingEmail
    ) {
        showAccount(
            existingEmail
        );
    }
</script>

</body>
</html>
`
