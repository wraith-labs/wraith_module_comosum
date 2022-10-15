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
                        <Password id="token" v-model="token" placeholder="Token" :toggleMask="true" class="w-full mb-3" inputClass="w-full" inputStyle="padding:1rem"></Password>

                        <Button label="Sign In" class="w-full p-3 text-xl"></button>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script>
export default {
    data() {
        return {
            token: '',
            remember: false
        }
    }
}
</script>
