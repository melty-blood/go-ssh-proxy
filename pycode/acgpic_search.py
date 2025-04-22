import cv2
import numpy as np
import os

# 计算两张图片之间的相似度
def match_images(target_img, search_img):
    # 初始化 ORB 检测器
    orb = cv2.ORB_create()

    # 提取目标图像的特征点和描述符
    kp1, des1 = orb.detectAndCompute(target_img, None)
    if des1 is None:
        return 0  # 无法计算描述符

    # 提取待搜索图像的特征点和描述符
    kp2, des2 = orb.detectAndCompute(search_img, None)
    if des2 is None:
        return 0  # 无法计算描述符

    # 使用暴力匹配器 (Brute Force Matcher)
    bf = cv2.BFMatcher(cv2.NORM_HAMMING, crossCheck=True)

    # 找到匹配的特征点
    matches = bf.match(des1, des2)

    # 按照匹配距离排序
    matches = sorted(matches, key = lambda x:x.distance)

    # 计算匹配点的数量
    match_count = len(matches)

    return match_count

# 在目录中搜索与目标图像相似的图像
def search_images_in_directory(target_img_path, search_directory):
    target_img = cv2.imread(target_img_path, cv2.IMREAD_COLOR)
    if target_img is None:
        print("无法读取目标图像")
        return
    
    best_match_count = 0
    best_match_image = None
    best_match_file = ""

    # 遍历搜索目录中的所有图像
    for file_name in os.listdir(search_directory):
        file_path = os.path.join(search_directory, file_name)

        # 只处理图像文件
        if file_path.endswith(('.jpg', '.png', '.jpeg')):
            search_img = cv2.imread(file_path, cv2.IMREAD_COLOR)
            if search_img is None:
                continue

            # 匹配图像
            match_count = match_images(target_img, search_img)

            # 如果当前匹配数更多，更新最佳匹配
            if match_count > 130:
                best_match_count = match_count
                best_match_image = search_img
                best_match_file = file_path
                print(f"最佳匹配文件：{best_match_file}，匹配特征点数量：{best_match_count}")

    # 输出最佳匹配
    # if best_match_count > 0:
    #     print(f"最佳匹配文件：{best_match_file}，匹配特征点数量：{best_match_count}")
    #     # cv2.imshow("Best Match", best_match_image)
    #     # cv2.waitKey(0)
    #     # cv2.destroyAllWindows()
    # else:
    #     print("没有找到匹配的图像")

# 主程序入口
if __name__ == "__main__":
    target_img_path = "/dev/shm/acg_image1.jpg"  # 目标图片路径
    search_directory = "/dev/shm/temp"  # 待搜索图片的目录
    search_images_in_directory(target_img_path, search_directory)
