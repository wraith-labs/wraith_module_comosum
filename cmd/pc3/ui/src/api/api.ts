import { sha512 } from './helpers'

const API_PATH_BASE = 'X/'
const API_PATH_AUTH = API_PATH_BASE+'auth'
const API_PATH_CHECKAUTH = API_PATH_BASE+'checkauth'
const API_PATH_CLIENTS = API_PATH_BASE+'clients'
const API_PATH_ABOUT = API_PATH_BASE+'about'

export default class API {

    async apifetch(input: RequestInfo | URL, init?: RequestInit | undefined): Promise<Response> {
        // If we don't have an active session, fallback to a regular fetch
        if (!this.checkauth()) {
            return fetch(input, init)
        }

        let session: {
            token: string,
            expiry: number,
            access: number,
        } = JSON.parse(localStorage.getItem('session') as any)

        let customInit: RequestInit = {}
        if (init !== undefined) {
            customInit = init
        }
        if (customInit.headers === undefined) {
            customInit.headers = {
                'Authorization': 'Bearer '+session.token,
            }
        } else {
            customInit.headers = {
                ...customInit.headers,
                'Authorization': 'Bearer '+session.token,
            }
        }

        return fetch(input, customInit)
    }

    async auth(uToken: string) {
        // Hash the token with a time-based nonce for slightly improved security.
        const salt = new Date().getTime()
        const uTokenHash = await sha512(uToken+'|'+salt+'|wmp')

        // Send the auth request.
        const res = await fetch(
            API_PATH_AUTH,
            {
                method: 'POST',
                body: JSON.stringify({
                    token: uTokenHash,
                    time: salt,
                }),
            })

        if (res.status !== 200) {
            return false
        }

        const resdata = await res.json()
        const sToken = resdata['token']
        const expiry = resdata['expiry']
        const access = resdata['access']

        if (sToken === undefined ||
            expiry === undefined ||
            access === undefined
        ) {
            return false
        }

        localStorage.setItem('session', JSON.stringify({
            'token': sToken,
            'expiry': expiry,
            'access': access,
        }));

        return true
    }

    async checkauth() {
        let session: {
            token: string,
            expiry: number,
            access: number,
        }
        try {
            session = JSON.parse(localStorage.getItem('session') as any)
        } catch {
            // Clear localStorage to make sure no unnecessary data hangs around.
            localStorage.clear()
            
            return false
        }

        // We don't have the session data; session invalid.
        if (session === null) {
            // Clear localStorage to make sure no unnecessary data hangs around.
            localStorage.clear()

            return false
        }

        const token = session['token']
        const expiry = session['expiry']

        // The token has expired; session invalid.
        if (new Date(expiry).getTime() < new Date().getTime()) {
            // Clear localStorage to make sure no unnecessary data hangs around.
            localStorage.clear()

            return false
        }

        // Finally, bounce the session against the API to be sure.
        // Do not use apifetch here or we'll get infinite recursion!
        const res = await fetch(
            API_PATH_CHECKAUTH,
            {
                method: 'POST',
                headers: {
                    'Authorization': 'Bearer '+token,
                },
                body: token,
            }
        )

        // Deliberately ignore other errors; we don't want to log the user out due to
        // poor network connectivity, temporary server downtime or internal server errors.
        if (res.status === 401) {
            // Clear localStorage to make sure no unnecessary data hangs around.
            localStorage.clear()

            return false
        }

        return true
    }

    async fetchClients(offset: number, limit: number) {
        const res = await this.apifetch(API_PATH_CLIENTS, {
            method: 'POST',
            body: JSON.stringify({
                offset,
                limit
            })
        })
        return res.json()
    }

    async fetchAbout() {
        const res = await this.apifetch(API_PATH_ABOUT)
        return res.json()
    }

}
  