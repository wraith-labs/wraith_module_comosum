<template>
	<div class="layout-topbar">
		<router-link to="/" class="layout-topbar-logo">
			<img alt="Logo" :src="topbarImage()" />
			<span>Wraith Pinecomms Admin</span>
		</router-link>
		<button class="p-link layout-menu-button layout-topbar-button" @click="onMenuToggle">
			<i class="pi pi-bars"></i>
		</button>

		<button class="p-link layout-topbar-menu-button layout-topbar-button"
			v-styleclass="{ selector: '@next', enterClass: 'hidden', enterActiveClass: 'scalein', 
			leaveToClass: 'hidden', leaveActiveClass: 'fadeout', hideOnOutsideClick: true}">
			<i class="pi pi-ellipsis-v"></i>
		</button>
		<ul class="layout-topbar-menu hidden lg:flex origin-top">
			<li>
				<button class="p-link layout-topbar-button">
					<i class="pi pi-cog"></i>
					<span>Settings</span>
				</button>
			</li>
			<li>
				<button class="p-link layout-topbar-button" @click="signout()">
					<i class="pi pi-sign-out"></i>
					<span>Sign Out</span>
				</button>
			</li>
		</ul>
	</div>
</template>

<script lang="ts">
import { defineComponent } from 'vue';

export default defineComponent({
  methods: {
		signout() {
			localStorage.clear()
			window.location.reload()
		},
        onMenuToggle(event: Event) {
            this.$emit('menu-toggle', event);
        },
		onTopbarMenuToggle(event: Event) {
            this.$emit('topbar-menu-toggle', event);
        },
		topbarImage(): string {
			return 'images/logo.png';
		}
    },
})
</script>