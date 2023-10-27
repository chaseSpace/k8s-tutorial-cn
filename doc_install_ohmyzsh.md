## 快速安装ohmyzsh

```shell
yum install -y git zsh wget

wget https://gitee.com/mirrors/oh-my-zsh/raw/master/tools/install.sh
vi install.sh
# 修改下面两行
# REPO=${REPO:-ohmyzsh/ohmyzsh}
# REMOTE=${REMOTE:-https://github.com/${REPO}.git}
# 为
# REPO=${REPO:-mirrors/oh-my-zsh}
# REMOTE=${REMOTE:-https://gitee.com/${REPO}.git}
# 保存 并 执行
chmod +x install.sh && ./install.sh

# 修改主题
ls ~/.oh-my-zsh/themes
vi ~/.zshrc
# 找到 ZSH_THEME 行，修改为自己想用的主题名称即可

# 安装插件
git clone https://gitee.com/jsharkc/zsh-autosuggestions.git $ZSH_CUSTOM/plugins/zsh-autosuggestions
git clone https://gitee.com/jsharkc/zsh-syntax-highlighting.git $ZSH_CUSTOM/plugins/zsh-syntax-highlighting

# 配置插件
sed -i 's/plugins=(git)/plugins=(git zsh-autosuggestions zsh-syntax-highlighting)/' ~/.zshrc
# 设置kubectl别名
echo 'alias kk="kubectl"' >> ~/.zshrc

# 生效
source ~/.zshrc
```