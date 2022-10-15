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
                        <label for="email1" class="block text-900 text-xl font-medium mb-2">Email</label>
                        <InputText id="email1" v-model="email" type="text" class="w-full mb-3" placeholder="Email" style="padding:1rem;" />
                
                        <label for="password1" class="block text-900 font-medium text-xl mb-2">Password</label>
                        <Password id="password1" v-model="password" placeholder="Password" :toggleMask="true" class="w-full mb-3" inputClass="w-full" inputStyle="padding:1rem"></Password>
                
                        <div class="flex align-items-center justify-content-between mb-5">
                            <div class="flex align-items-center">
                                <Checkbox id="rememberme1" v-model="remember" :binary="true" class="mr-2"></Checkbox>
                                <label for="rememberme1">Remember me</label>
                            </div>
                        </div>
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
            email: '',
            password: '',
            remember: false
        }
    }
}
</script>
