import { sha512 } from './helpers'

const API_PATH_BASE = 'X/'
const API_PATH_AUTH = API_PATH_BASE+'auth'
const API_PATH_CHECKAUTH = API_PATH_BASE+'checkauth'

export default class API {

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

}
  