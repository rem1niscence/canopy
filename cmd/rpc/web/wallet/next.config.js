/** @type {import('next').NextConfig} */
const nextConfig = {
    output: "export",
    basePath: process.env.WALLET_BASE_PATH || '',  // default to root if no wallet base path
};

module.exports = nextConfig;