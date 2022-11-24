const API_BASE_PATH = 'X/'
const API_AUTH_PATH = API_BASE_PATH+'auth'
const API_CHECKAUTH_PATH = API_BASE_PATH+'checkauth'

export default class API {

    async auth(uToken) {
        const res = await fetch(API_AUTH_PATH, { method: 'POST', body: uToken })

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
        let session = {}
        try {
            session = JSON.parse(localStorage.getItem('session'))
        } catch {
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
        const access = session['access']

        // We're missing some session information; session invalid.
        if (token === undefined ||
            expiry === undefined ||
            access === undefined
        ) {
            // Clear localStorage to make sure no unnecessary data hangs around.
            localStorage.clear()

            return false
        }

        // The token has expired; session invalid.
        if (new Date(expiry) < new Date().getUTCDate()) {
            // Clear localStorage to make sure no unnecessary data hangs around.
            localStorage.clear()

            return false
        }

        // Finally, bounce the session against the API to be sure.
        const res = await fetch(
            API_CHECKAUTH_PATH,
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
  