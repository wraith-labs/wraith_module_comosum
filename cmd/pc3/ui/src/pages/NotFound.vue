<template>
    <div class="surface-0 flex align-items-center justify-content-center min-h-screen min-w-screen overflow-hidden">
        <div class="grid justify-content-center p-2 lg:p-0" style="min-width:80%">
            <div class="col-12 mt-5 xl:mt-0 text-center">
                <img src="images/logo.png" alt="Wraith logo" class="mb-5" style="width:80px">
            </div>
            <div class="col-12 xl:col-6" style="border-radius:56px; padding:0.3rem; background: linear-gradient(180deg, rgba(218, 205, 33, 0.4) 10%, rgba(218, 214, 18, 0) 30%);">
                <div class="flex justify-content-center h-full w-full m-0 py-7 px-4" style="border-radius:53px; background: linear-gradient(180deg, var(--surface-50) 38.9%, var(--surface-0));">
                    <div class="grid flex-column align-items-center">
                        <span class="text-yellow-500 font-bold text-3xl">404</span>
                        <h1 class="text-900 font-bold text-3xl lg:text-5xl mb-2">Looks like you are lost</h1>
                        <span class="text-gray-600">Requested resource is not available.</span>
                        <div class="col-12 mt-5 text-center">
                            <i class="pi pi-fw pi-arrow-left text-yellow-500 mr-2" style="vertical-align:center;"></i>
                            <router-link to="/" class="text-yellow-500">Go to Dashboard</router-link>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>