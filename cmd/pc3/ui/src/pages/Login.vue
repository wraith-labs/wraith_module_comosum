<template>
    <div class="surface-0 flex align-items-center justify-content-center min-h-screen min-w-screen overflow-hidden">
        <div class="grid justify-content-center p-2 lg:p-0" style="min-width:80%">
            <div class="col-12 mt-5 xl:mt-0 text-center">
                <img src="images/logo.png" alt="Wraith logo" class="mb-5" style="width:80px">
            </div>
            <div class="col-12 xl:col-6" style="border-radius:56px; padding:0.3rem; background: linear-gradient(180deg, var(--primary-color), rgba(33, 150, 243, 0) 30%);">
                <div class="h-full w-full m-0 py-7 px-4" style="border-radius:53px; background: linear-gradient(180deg, var(--surface-50) 38.9%, var(--surface-0));">
                    <div class="text-center mb-5">
                        <h3 class="text-600">Sign in to continue</h3>
                    </div>
                
                    <div class="w-full md:w-10 mx-auto">
                        <label for="token" class="block text-900 font-medium text-xl mb-2">Token</label>
                        <small id="token-auth-failed-msg" class="p-error" :hidden="true">Token auth failed</small>
                        <Password
                            id="token"
                            v-model="token"
                            placeholder="Enter Token"
                            class="w-full mb-3"
                            inputClass="w-full"
                            inputStyle="padding:1rem"
                            :toggleMask="true"
                            :feedback="false"
                            :required="true"
                            @keyup.enter = "signIn"
                        ></Password>

                        <Button label="Sign In" class="w-full p-3 text-xl" @click="signIn"></button>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script>
import API from '../api/api';

export default {
    created() {
        this.api = new API()
    },
    beforeMount() {
        this.api.checkauth().then((authed) => {
            if (authed) {
                window.location.hash = '#/'
            }
        })
    },
    data() {
        return {
            token: '',
        }
    },
    methods: {
        signIn() {
            this.api.auth(this.token).then((authed) => {
                if (authed) {
                    document.getElementById('token-auth-failed-msg').hidden = true
                    window.location.hash = '#/'
                } else {
                    document.getElementById('token-auth-failed-msg').hidden = false
                }
            })
        }
    }
}
</script>
