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
	page :=
		template.Must(
			template.New(
				"index",
			).Parse(
				indexHTML,
			),
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

	err :=
		handler.page.Execute(
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

            padding: 32px;

            background: white;

            border-radius: 16px;

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
        .button-link {
            width: 100%;

            display: block;

            padding: 12px 16px;

            border: 0;
            border-radius: 8px;

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

        .telegram-link {
            margin-top: 14px;

            background: #229ed9;
            color: white;
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

        .link-result {
            display: none;

            margin-top: 20px;

            padding: 16px;

            background: #f3f4f6;

            border-radius: 8px;
        }

        .link-result-title {
            margin-bottom: 8px;

            font-size: 14px;
            font-weight: 600;
        }

        .link-code {
            margin: 0;

            font-size: 21px;
            font-weight: 700;
        }

        .hint {
            margin-top: 14px;

            font-size: 13px;

            line-height: 1.5;

            color: #6b7280;
        }

        .command {
            display: inline-block;

            margin-top: 6px;

            padding: 4px 6px;

            border-radius: 4px;

            background: #e5e7eb;

            color: #111827;

            font-family: monospace;
        }
    </style>
</head>

<body>

<div class="page">

    <main class="card">

        <section id="authSection">

            <h1>
                MovieTracker
            </h1>

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

            <h1>
                MovieTracker
            </h1>

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
                class="link-result"
            >

                <div class="link-result-title">
                    Код привязки
                </div>

                <p
                    id="telegramCode"
                    class="link-code"
                ></p>

                <a
                    id="telegramLink"
                    class="button-link telegram-link"
                    href="#"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    Открыть бота в Telegram
                </a>

                <div class="hint">

                    MovieTracker останется открыт
                    в этой вкладке.

                    <br><br>

                    Если Telegram не открыл бота,
                    найди

                    <strong>
                        @{{ .TelegramBotUsername }}
                    </strong>

                    вручную и отправь:

                    <br>

                    <span
                        id="telegramCommand"
                        class="command"
                    ></span>

                </div>

            </div>

            <div
                id="accountMessage"
                class="message"
            ></div>

            <button
                id="logout"
                class="secondary"
                type="button"
            >
                Выйти
            </button>

        </section>

    </main>

</div>


<script>

    const telegramBotUsername =
        {{ printf "%q" .TelegramBotUsername }};


    const accessTokenKey =
        "movietracker_access_token";

    const emailKey =
        "movietracker_email";


    const authSection =
        document.getElementById(
            "authSection"
        );

    const accountSection =
        document.getElementById(
            "accountSection"
        );

    const authForm =
        document.getElementById(
            "authForm"
        );

    const emailInput =
        document.getElementById(
            "email"
        );

    const passwordInput =
        document.getElementById(
            "password"
        );

    const submitButton =
        document.getElementById(
            "submitButton"
        );

    const switchModeButton =
        document.getElementById(
            "switchMode"
        );

    const switchText =
        document.getElementById(
            "switchText"
        );

    const authSubtitle =
        document.getElementById(
            "authSubtitle"
        );

    const authMessage =
        document.getElementById(
            "authMessage"
        );

    const accountMessage =
        document.getElementById(
            "accountMessage"
        );

    const accountEmail =
        document.getElementById(
            "accountEmail"
        );

    const connectTelegramButton =
        document.getElementById(
            "connectTelegram"
        );

    const telegramResult =
        document.getElementById(
            "telegramResult"
        );

    const telegramCode =
        document.getElementById(
            "telegramCode"
        );

    const telegramCommand =
        document.getElementById(
            "telegramCommand"
        );

    const telegramLink =
        document.getElementById(
            "telegramLink"
        );

    const logoutButton =
        document.getElementById(
            "logout"
        );


    let mode = "login";


    function setMessage(
        element,
        text,
        type
    ) {
        element.textContent =
            text;

        element.className =
            "message " + type;
    }


    function clearMessage(
        element
    ) {
        element.textContent =
            "";

        element.className =
            "message";
    }


    function showAccount(
        email
    ) {
        authSection.style.display =
            "none";

        accountSection.style.display =
            "block";

        accountEmail.textContent =
            email;

        clearMessage(
            accountMessage
        );
    }


    function showAuth() {
        accountSection.style.display =
            "none";

        authSection.style.display =
            "block";

        telegramResult.style.display =
            "none";

        clearMessage(
            authMessage
        );
    }


    function getToken() {
        return sessionStorage.getItem(
            accessTokenKey
        );
    }


    function saveAccessToken(
        token
    ) {
        sessionStorage.setItem(
            accessTokenKey,
            token
        );
    }


    function saveSession(
        token,
        email
    ) {
        saveAccessToken(
            token
        );

        sessionStorage.setItem(
            emailKey,
            email
        );
    }


    function clearSession() {
        sessionStorage.removeItem(
            accessTokenKey
        );

        sessionStorage.removeItem(
            emailKey
        );
    }


    async function requestJSON(
        url,
        options = {}
    ) {
        const response =
            await fetch(
                url,
                {
                    ...options,

                    credentials:
                        "same-origin"
                }
            );

        let data = null;

        try {
            data =
                await response.json();
        } catch (_) {
        }

        if (!response.ok) {

            const message =
                data?.error?.message ||
                "HTTP " +
                response.status;

            const error =
                new Error(
                    message
                );

            error.status =
                response.status;

            throw error;
        }

        return data;
    }


    async function refreshAccessToken() {
        try {

            const data =
                await requestJSON(
                    "/auth/refresh",
                    {
                        method:
                            "POST"
                    }
                );

            if (!data?.access_token) {
                throw new Error(
                    "Auth service did not return access token"
                );
            }

            saveAccessToken(
                data.access_token
            );

            return data.access_token;

        } catch (_) {

            clearSession();

            return null;
        }
    }


    function sessionExpiredError() {
        const error =
            new Error(
                "Сессия истекла. Войди снова."
            );

        error.sessionExpired =
            true;

        return error;
    }


    async function authorizedRequestJSON(
        url,
        options = {}
    ) {
        let token =
            getToken();

        if (!token) {
            token =
                await refreshAccessToken();

            if (!token) {
                throw sessionExpiredError();
            }
        }

        function optionsWithToken(
            accessToken
        ) {
            const headers =
                new Headers(
                    options.headers || {}
                );

            headers.set(
                "Authorization",
                "Bearer " +
                    accessToken
            );

            return {
                ...options,
                headers
            };
        }

        try {

            return await requestJSON(
                url,
                optionsWithToken(
                    token
                )
            );

        } catch (error) {

            if (
                error.status !== 401
            ) {
                throw error;
            }
        }

        token =
            await refreshAccessToken();

        if (!token) {
            throw sessionExpiredError();
        }

        return requestJSON(
            url,
            optionsWithToken(
                token
            )
        );
    }


    async function login(
        email,
        password
    ) {
        const data =
            await requestJSON(
                "/auth/login",
                {
                    method:
                        "POST",

                    headers: {
                        "Content-Type":
                            "application/json"
                    },

                    body:
                        JSON.stringify({
                            email,
                            password
                        })
                }
            );

        if (!data?.access_token) {
            throw new Error(
                "Auth service did not return access token"
            );
        }

        saveSession(
            data.access_token,
            email
        );

        showAccount(
            email
        );
    }


    async function register(
        email,
        password
    ) {
        await requestJSON(
            "/auth/register",
            {
                method:
                    "POST",

                headers: {
                    "Content-Type":
                        "application/json"
                },

                body:
                    JSON.stringify({
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

        async function(
            event
        ) {
            event.preventDefault();

            clearMessage(
                authMessage
            );

            const email =
                emailInput.value.trim();

            const password =
                passwordInput.value;

            submitButton.disabled =
                true;

            try {

                if (
                    mode === "login"
                ) {
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

                submitButton.disabled =
                    false;
            }
        }
    );


    switchModeButton.addEventListener(
        "click",

        function() {

            clearMessage(
                authMessage
            );

            if (
                mode === "login"
            ) {
                mode =
                    "register";

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

                mode =
                    "login";

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

            clearMessage(
                accountMessage
            );

            telegramResult.style.display =
                "none";

            connectTelegramButton.disabled =
                true;

            try {

                const data =
                    await authorizedRequestJSON(
                        "/auth/telegram/link-code",
                        {
                            method:
                                "POST"
                        }
                    );

                if (!data?.code) {
                    throw new Error(
                        "Auth service did not return link code"
                    );
                }

                telegramCode.textContent =
                    data.code;

                telegramCommand.textContent =
                    "/link " +
                    data.code;

                const startParameter =
                    "link_" +
                    data.code;

                telegramLink.href =
                    "https://t.me/" +
                    telegramBotUsername +
                    "?start=" +
                    encodeURIComponent(
                        startParameter
                    );

                telegramResult.style.display =
                    "block";

                setMessage(
                    accountMessage,
                    "Ссылка создана. Нажми «Открыть бота в Telegram» и затем Start.",
                    "success"
                );

            } catch (error) {

                if (
                    error.sessionExpired
                ) {
                    clearSession();

                    showAuth();

                    setMessage(
                        authMessage,
                        error.message,
                        "error"
                    );

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

        async function() {

            logoutButton.disabled =
                true;

            clearMessage(
                accountMessage
            );

            let logoutError = null;

            try {

                await requestJSON(
                    "/auth/logout",
                    {
                        method:
                            "POST"
                    }
                );

            } catch (error) {

                logoutError =
                    error;

            } finally {

                clearSession();

                showAuth();

                logoutButton.disabled =
                    false;
            }

            if (logoutError) {
                setMessage(
                    authMessage,
                    "Локальная сессия завершена, но сервер не подтвердил выход: " +
                        logoutError.message,
                    "error"
                );
            }
        }
    );


    async function restorePage() {
        const existingToken =
            getToken();

        const existingEmail =
            sessionStorage.getItem(
                emailKey
            );

        if (
            existingToken &&
            existingEmail
        ) {
            showAccount(
                existingEmail
            );

            return;
        }

        showAuth();
    }


    restorePage();

</script>

</body>

</html>
`
